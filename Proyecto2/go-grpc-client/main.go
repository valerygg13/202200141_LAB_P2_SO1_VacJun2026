package main

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"os"
	"time"

	pb "quiniela/proto/worldcuppb"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

// MatchPrediction representa el JSON recibido desde la API REST Go.
type MatchPrediction struct {
	HomeTeam  string `json:"home_team"`
	AwayTeam  string `json:"away_team"`
	HomeGoals int32  `json:"home_goals"`
	AwayGoals int32  `json:"away_goals"`
	Username  string `json:"username"`
	Timestamp string `json:"timestamp"`
}

// APIResponse representa la respuesta HTTP del cliente gRPC.
type APIResponse struct {
	Status  string `json:"status"`
	Message string `json:"message"`
}

// application contiene el cliente generado por Protocol Buffers.
type application struct {
	grpcClient pb.MatchPredictionServiceClient
}

func main() {
	grpcServerAddress := os.Getenv("GRPC_SERVER_ADDR")
	if grpcServerAddress == "" {
		grpcServerAddress = "localhost:50051"
	}

	// Crea el canal de comunicación con el servidor gRPC.
	connection, err := grpc.NewClient(
		grpcServerAddress,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		log.Fatalf("No se pudo crear el cliente gRPC: %v", err)
	}
	defer connection.Close()

	app := &application{
		grpcClient: pb.NewMatchPredictionServiceClient(connection),
	}

	mux := http.NewServeMux()
	mux.HandleFunc("GET /health", app.healthHandler)
	mux.HandleFunc("POST /send", app.sendPredictionHandler)

	server := &http.Server{
		Addr:              ":8083",
		Handler:           mux,
		ReadHeaderTimeout: 5 * time.Second,
	}

	log.Println("Cliente gRPC ejecutándose en http://localhost:8083")
	log.Println("Ruta de salud: GET /health")
	log.Println("Ruta principal: POST /send")
	log.Printf("Servidor gRPC configurado: %s", grpcServerAddress)

	if err := server.ListenAndServe(); err != nil &&
		err != http.ErrServerClosed {
		log.Fatalf("Error al ejecutar el cliente gRPC: %v", err)
	}
}

func (app *application) healthHandler(
	w http.ResponseWriter,
	_ *http.Request,
) {
	writeJSON(w, http.StatusOK, APIResponse{
		Status:  "ok",
		Message: "El cliente gRPC está funcionando",
	})
}

func (app *application) sendPredictionHandler(
	w http.ResponseWriter,
	r *http.Request,
) {
	defer r.Body.Close()

	var prediction MatchPrediction

	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()

	if err := decoder.Decode(&prediction); err != nil {
		writeJSON(w, http.StatusBadRequest, APIResponse{
			Status:  "error",
			Message: "El cuerpo no contiene un JSON válido",
		})
		return
	}

	homeTeam, validHomeTeam := teamToProto(prediction.HomeTeam)
	awayTeam, validAwayTeam := teamToProto(prediction.AwayTeam)

	if !validHomeTeam || !validAwayTeam {
		writeJSON(w, http.StatusBadRequest, APIResponse{
			Status:  "error",
			Message: "El equipo local o visitante no es válido",
		})
		return
	}

	// Evita que una llamada quede esperando indefinidamente.
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	response, err := app.grpcClient.SendPrediction(
		ctx,
		&pb.MatchPredictionRequest{
			HomeTeam:  homeTeam,
			AwayTeam:  awayTeam,
			HomeGoals: prediction.HomeGoals,
			AwayGoals: prediction.AwayGoals,
			Username:  prediction.Username,
			Timestamp: prediction.Timestamp,
		},
	)
	if err != nil {
		log.Printf("La llamada gRPC falló: %v", err)

		writeJSON(w, http.StatusBadGateway, APIResponse{
			Status:  "error",
			Message: "No se pudo procesar la predicción mediante gRPC",
		})
		return
	}

	log.Printf("Respuesta del servidor gRPC: %s", response.GetStatus())

	writeJSON(w, http.StatusOK, APIResponse{
		Status:  "success",
		Message: response.GetStatus(),
	})
}

// teamToProto convierte el texto recibido por HTTP al enum del archivo .proto.
func teamToProto(team string) (pb.Teams, bool) {
	switch team {
	case "GTM":
		return pb.Teams_GTM, true
	case "MEX":
		return pb.Teams_MEX, true
	case "BRA":
		return pb.Teams_BRA, true
	case "ARG":
		return pb.Teams_ARG, true
	case "ESP":
		return pb.Teams_ESP, true
	default:
		return pb.Teams_TEAMS_UNKNOWN, false
	}
}

func writeJSON(
	w http.ResponseWriter,
	statusCode int,
	response APIResponse,
) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)

	if err := json.NewEncoder(w).Encode(response); err != nil {
		log.Printf("No se pudo escribir la respuesta JSON: %v", err)
	}
}
