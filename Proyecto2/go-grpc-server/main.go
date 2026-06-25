package main

import (
	"context"
	"log"
	"net"

	pb "quiniela/proto/worldcuppb"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// predictionServer implementa el servicio definido en worldcup.proto.
type predictionServer struct {
	pb.UnimplementedMatchPredictionServiceServer
}

// SendPrediction recibe una predicción enviada por el cliente gRPC.
func (s *predictionServer) SendPrediction(
	_ context.Context,
	request *pb.MatchPredictionRequest,
) (*pb.MatchPredictionResponse, error) {
	// Validar los equipos.
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

	// Validar los goles.
	if request.GetHomeGoals() < 0 ||
		request.GetHomeGoals() > 5 ||
		request.GetAwayGoals() < 0 ||
		request.GetAwayGoals() > 5 {
		return nil, status.Error(
			codes.InvalidArgument,
			"los goles deben estar entre 0 y 5",
		)
	}

	if request.GetUsername() == "" || request.GetTimestamp() == "" {
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

	// Más adelante aquí enviaremos la predicción al RabbitMQ Writer.
	return &pb.MatchPredictionResponse{
		Status: "Predicción recibida correctamente por el servidor gRPC",
	}, nil
}

func main() {
	const address = ":50051"

	listener, err := net.Listen("tcp", address)
	if err != nil {
		log.Fatalf("No se pudo abrir el puerto %s: %v", address, err)
	}

	grpcServer := grpc.NewServer()

	pb.RegisterMatchPredictionServiceServer(
		grpcServer,
		&predictionServer{},
	)

	log.Printf("Servidor gRPC ejecutándose en localhost%s", address)
	log.Println("Servicio: worldcup2026.MatchPredictionService")
	log.Println("Método: SendPrediction")

	if err := grpcServer.Serve(listener); err != nil {
		log.Fatalf("Error al ejecutar el servidor gRPC: %v", err)
	}
}
