package main

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"log"
	"net"
	"net/http"
	"os"
	"time"

	pb "quiniela/proto/worldcuppb"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// MatchPredictionPayload representa el JSON enviado al RabbitMQ Writer.
type MatchPredictionPayload struct {
	HomeTeam  string `json:"home_team"`
	AwayTeam  string `json:"away_team"`
	HomeGoals int32  `json:"home_goals"`
	AwayGoals int32  `json:"away_goals"`
	Username  string `json:"username"`
	Timestamp string `json:"timestamp"`
}

// WriterResponse representa la respuesta del RabbitMQ Writer.
type WriterResponse struct {
	Status  string `json:"status"`
	Message string `json:"message"`
}

// predictionServer implementa el servicio definido en worldcup.proto.
type predictionServer struct {
	pb.UnimplementedMatchPredictionServiceServer

	httpClient      *http.Client
	rabbitWriterURL string
}

// SendPrediction recibe una predicción desde el cliente gRPC
// y la envía al RabbitMQ Writer.
func (server *predictionServer) SendPrediction(
	ctx context.Context,
	request *pb.MatchPredictionRequest,
) (*pb.MatchPredictionResponse, error) {
	if request.GetHomeTeam() == pb.Teams_TEAMS_UNKNOWN ||
		request.GetAwayTeam() == pb.Teams_TEAMS_UNKNOWN {
		return nil, status.Error(
			codes.InvalidArgument,
			"el equipo local o visitante no es válido",
		)
	}

	if request.GetHomeTeam() == request.GetAwayTeam() {
		return nil, status.Error(
			codes.InvalidArgument,
			"el equipo local y visitante deben ser diferentes",
		)
	}

	if request.GetHomeGoals() < 0 ||
		request.GetHomeGoals() > 5 ||
		request.GetAwayGoals() < 0 ||
		request.GetAwayGoals() > 5 {
		return nil, status.Error(
			codes.InvalidArgument,
			"los goles deben estar entre 0 y 5",
		)
	}

	if request.GetUsername() == "" ||
		request.GetTimestamp() == "" {
		return nil, status.Error(
			codes.InvalidArgument,
			"el usuario y timestamp son obligatorios",
		)
	}

	log.Println("Predicción recibida por el servidor gRPC:")
	log.Printf(
		"%s %d - %d %s | usuario: %s | timestamp: %s",
		request.GetHomeTeam().String(),
		request.GetHomeGoals(),
		request.GetAwayGoals(),
		request.GetAwayTeam().String(),
		request.GetUsername(),
		request.GetTimestamp(),
	)

	payload := MatchPredictionPayload{
		HomeTeam:  request.GetHomeTeam().String(),
		AwayTeam:  request.GetAwayTeam().String(),
		HomeGoals: request.GetHomeGoals(),
		AwayGoals: request.GetAwayGoals(),
		Username:  request.GetUsername(),
		Timestamp: request.GetTimestamp(),
	}

	jsonBody, err := json.Marshal(payload)
	if err != nil {
		log.Printf("No se pudo convertir la predicción a JSON: %v", err)

		return nil, status.Error(
			codes.Internal,
			"no se pudo preparar la predicción",
		)
	}

	writerContext, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	httpRequest, err := http.NewRequestWithContext(
		writerContext,
		http.MethodPost,
		server.rabbitWriterURL,
		bytes.NewReader(jsonBody),
	)
	if err != nil {
		log.Printf("No se pudo crear la petición al Writer: %v", err)

		return nil, status.Error(
			codes.Internal,
			"no se pudo crear la petición al RabbitMQ Writer",
		)
	}

	httpRequest.Header.Set("Content-Type", "application/json")

	httpResponse, err := server.httpClient.Do(httpRequest)
	if err != nil {
		log.Printf("No se pudo conectar con RabbitMQ Writer: %v", err)

		return nil, status.Error(
			codes.Unavailable,
			"no se pudo conectar con RabbitMQ Writer",
		)
	}
	defer httpResponse.Body.Close()

	responseBody, err := io.ReadAll(httpResponse.Body)
	if err != nil {
		log.Printf("No se pudo leer la respuesta del Writer: %v", err)

		return nil, status.Error(
			codes.Unavailable,
			"respuesta inválida del RabbitMQ Writer",
		)
	}

	if httpResponse.StatusCode < 200 ||
		httpResponse.StatusCode >= 300 {
		log.Printf(
			"RabbitMQ Writer respondió HTTP %d: %s",
			httpResponse.StatusCode,
			string(responseBody),
		)

		return nil, status.Error(
			codes.Unavailable,
			"RabbitMQ Writer no pudo publicar la predicción",
		)
	}

	var writerResponse WriterResponse

	if err := json.Unmarshal(responseBody, &writerResponse); err != nil {
		log.Printf("El Writer devolvió un JSON inválido: %v", err)

		return nil, status.Error(
			codes.Unavailable,
			"respuesta inválida del RabbitMQ Writer",
		)
	}

	if writerResponse.Message == "" {
		writerResponse.Message =
			"Predicción publicada correctamente en RabbitMQ"
	}

	log.Printf(
		"Respuesta del RabbitMQ Writer: %s",
		writerResponse.Message,
	)

	return &pb.MatchPredictionResponse{
		Status: writerResponse.Message,
	}, nil
}

func main() {
	const grpcAddress = ":50051"

	rabbitWriterURL := os.Getenv("RABBIT_WRITER_URL")
	if rabbitWriterURL == "" {
		rabbitWriterURL = "http://localhost:8084/publish"
	}

	listener, err := net.Listen("tcp", grpcAddress)
	if err != nil {
		log.Fatalf(
			"No se pudo abrir el puerto %s: %v",
			grpcAddress,
			err,
		)
	}

	grpcServer := grpc.NewServer()

	server := &predictionServer{
		httpClient: &http.Client{
			Timeout: 6 * time.Second,
		},
		rabbitWriterURL: rabbitWriterURL,
	}

	pb.RegisterMatchPredictionServiceServer(
		grpcServer,
		server,
	)

	log.Printf(
		"Servidor gRPC ejecutándose en localhost%s",
		grpcAddress,
	)
	log.Println("Servicio: worldcup2026.MatchPredictionService")
	log.Println("Método: SendPrediction")
	log.Printf("RabbitMQ Writer configurado: %s", rabbitWriterURL)

	if err := grpcServer.Serve(listener); err != nil {
		log.Fatalf(
			"Error al ejecutar el servidor gRPC: %v",
			err,
		)
	}
}
