package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"time"
)

// MatchPrediction representa una predicción recibida en formato JSON.
type MatchPrediction struct {
	HomeTeam  string `json:"home_team"`
	AwayTeam  string `json:"away_team"`
	HomeGoals int    `json:"home_goals"`
	AwayGoals int    `json:"away_goals"`
	Username  string `json:"username"`
	Timestamp string `json:"timestamp"`
}

// APIResponse representa la respuesta enviada por la API Go.
type APIResponse struct {
	Status  string `json:"status"`
	Message string `json:"message"`
}

func main() {
	// El ServeMux administra las rutas HTTP de la aplicación.
	mux := http.NewServeMux()

	// Rutas disponibles.
	mux.HandleFunc("GET /health", healthHandler)
	mux.HandleFunc("POST /prediction", predictionHandler)

	// Configuración del servidor.
	server := &http.Server{
		Addr:              ":8082",
		Handler:           mux,
		ReadHeaderTimeout: 5 * time.Second,
	}

	log.Println("API REST Go ejecutándose en http://localhost:8082")
	log.Println("Ruta de salud: GET /health")
	log.Println("Ruta principal: POST /prediction")

	// Inicia el servidor y lo mantiene escuchando peticiones.
	if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Fatalf("Error al ejecutar la API Go: %v", err)
	}
}

// healthHandler comprueba que el servicio está funcionando.
func healthHandler(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, APIResponse{
		Status:  "ok",
		Message: "La API REST Go está funcionando",
	})
}

// predictionHandler recibe una predicción enviada mediante POST.
func predictionHandler(w http.ResponseWriter, r *http.Request) {
	defer r.Body.Close()

	var prediction MatchPrediction

	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()

	// Convierte el JSON recibido en una estructura de Go.
	if err := decoder.Decode(&prediction); err != nil {
		writeJSON(w, http.StatusBadRequest, APIResponse{
			Status:  "error",
			Message: "El cuerpo de la petición no contiene un JSON válido",
		})
		return
	}

	// Validación básica.
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

	// Por ahora solo imprimimos la predicción.
	// Más adelante este servicio la enviará al cliente gRPC.
	fmt.Println("Predicción recibida por la API Go:")
	fmt.Printf("%+v\n", prediction)

	writeJSON(w, http.StatusOK, APIResponse{
		Status:  "success",
		Message: "Predicción recibida por la API Go",
	})
}

// writeJSON escribe una respuesta HTTP en formato JSON.
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
