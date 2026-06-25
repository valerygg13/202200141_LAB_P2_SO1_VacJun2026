package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"time"
)

// MatchPrediction representa el JSON recibido desde Rust.
type MatchPrediction struct {
	HomeTeam  string `json:"home_team"`
	AwayTeam  string `json:"away_team"`
	HomeGoals int    `json:"home_goals"`
	AwayGoals int    `json:"away_goals"`
	Username  string `json:"username"`
	Timestamp string `json:"timestamp"`
}

// APIResponse representa una respuesta HTTP en formato JSON.
type APIResponse struct {
	Status  string `json:"status"`
	Message string `json:"message"`
}

// application contiene los recursos compartidos por los handlers.
type application struct {
	httpClient    *http.Client
	grpcClientURL string
}

func main() {
	// En local utiliza localhost.
	// En Kubernetes se cambiará mediante una variable de entorno.
	grpcClientURL := os.Getenv("GRPC_CLIENT_URL")
	if grpcClientURL == "" {
		grpcClientURL = "http://localhost:8083/send"
	}

	app := &application{
		httpClient: &http.Client{
			Timeout: 6 * time.Second,
		},
		grpcClientURL: grpcClientURL,
	}

	mux := http.NewServeMux()
	mux.HandleFunc("GET /health", app.healthHandler)
	mux.HandleFunc("POST /prediction", app.predictionHandler)

	server := &http.Server{
		Addr:              ":8082",
		Handler:           mux,
		ReadHeaderTimeout: 5 * time.Second,
	}

	log.Println("API REST Go ejecutándose en http://localhost:8082")
	log.Println("Ruta de salud: GET /health")
	log.Println("Ruta principal: POST /prediction")
	log.Printf("Cliente gRPC configurado: %s", grpcClientURL)

	if err := server.ListenAndServe(); err != nil &&
		err != http.ErrServerClosed {
		log.Fatalf("Error al ejecutar la API Go: %v", err)
	}
}

func (app *application) healthHandler(
	w http.ResponseWriter,
	_ *http.Request,
) {
	writeJSON(w, http.StatusOK, APIResponse{
		Status:  "ok",
		Message: "La API REST Go está funcionando",
	})
}

func (app *application) predictionHandler(
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

	if prediction.HomeTeam == "" ||
		prediction.AwayTeam == "" ||
		prediction.Username == "" ||
		prediction.Timestamp == "" {

		writeJSON(w, http.StatusBadRequest, APIResponse{
			Status:  "error",
			Message: "Faltan campos obligatorios",
		})
		return
	}

	payload, err := json.Marshal(prediction)
	if err != nil {
		log.Printf("No se pudo convertir la predicción a JSON: %v", err)

		writeJSON(w, http.StatusInternalServerError, APIResponse{
			Status:  "error",
			Message: "No se pudo procesar la predicción",
		})
		return
	}

	request, err := http.NewRequestWithContext(
		r.Context(),
		http.MethodPost,
		app.grpcClientURL,
		bytes.NewReader(payload),
	)
	if err != nil {
		log.Printf("No se pudo crear la petición al cliente gRPC: %v", err)

		writeJSON(w, http.StatusInternalServerError, APIResponse{
			Status:  "error",
			Message: "No se pudo crear la petición interna",
		})
		return
	}

	request.Header.Set("Content-Type", "application/json")

	response, err := app.httpClient.Do(request)
	if err != nil {
		log.Printf("No se pudo conectar con el cliente gRPC: %v", err)

		writeJSON(w, http.StatusBadGateway, APIResponse{
			Status:  "error",
			Message: "No se pudo conectar con el cliente gRPC",
		})
		return
	}
	defer response.Body.Close()

	responseBody, err := io.ReadAll(response.Body)
	if err != nil {
		log.Printf("No se pudo leer la respuesta del cliente gRPC: %v", err)

		writeJSON(w, http.StatusBadGateway, APIResponse{
			Status:  "error",
			Message: "Respuesta inválida del cliente gRPC",
		})
		return
	}

	if response.StatusCode < 200 || response.StatusCode >= 300 {
		log.Printf(
			"El cliente gRPC respondió HTTP %d: %s",
			response.StatusCode,
			string(responseBody),
		)

		writeJSON(w, http.StatusBadGateway, APIResponse{
			Status:  "error",
			Message: "El cliente gRPC no pudo procesar la predicción",
		})
		return
	}

	var grpcResponse APIResponse

	if err := json.Unmarshal(responseBody, &grpcResponse); err != nil {
		log.Printf("Respuesta JSON inválida del cliente gRPC: %v", err)

		writeJSON(w, http.StatusBadGateway, APIResponse{
			Status:  "error",
			Message: "El cliente gRPC devolvió una respuesta inválida",
		})
		return
	}

	fmt.Println("Predicción recibida desde Rust:")
	fmt.Printf("%+v\n", prediction)
	log.Printf("Respuesta del flujo gRPC: %s", grpcResponse.Message)

	writeJSON(w, http.StatusOK, APIResponse{
		Status:  "success",
		Message: grpcResponse.Message,
	})
}

func writeJSON(
	w http.ResponseWriter,
	statusCode int,
	response APIResponse,
) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(statusCode)

	if err := json.NewEncoder(w).Encode(response); err != nil {
		log.Printf("No se pudo escribir la respuesta JSON: %v", err)
	}
}
