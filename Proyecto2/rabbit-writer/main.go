package main

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"os"
	"sync"
	"time"

	amqp "github.com/rabbitmq/amqp091-go"
)

// MatchPrediction representa la predicción recibida desde el servidor gRPC.
type MatchPrediction struct {
	HomeTeam  string `json:"home_team"`
	AwayTeam  string `json:"away_team"`
	HomeGoals int32  `json:"home_goals"`
	AwayGoals int32  `json:"away_goals"`
	Username  string `json:"username"`
	Timestamp string `json:"timestamp"`
}

// APIResponse representa una respuesta HTTP en formato JSON.
type APIResponse struct {
	Status  string `json:"status"`
	Message string `json:"message"`
}

// application contiene la conexión con RabbitMQ.
type application struct {
	connection *amqp.Connection
	channel    *amqp.Channel
	queue      amqp.Queue

	// Evita publicaciones simultáneas sobre el mismo canal.
	publishMutex sync.Mutex
}

func main() {
	rabbitMQURL := getEnvironmentVariable(
		"RABBITMQ_URL",
		"amqp://project2:project2pass@localhost:5672/",
	)

	queueName := getEnvironmentVariable(
		"RABBITMQ_QUEUE",
		"predictions",
	)

	connection, err := amqp.Dial(rabbitMQURL)
	if err != nil {
		log.Fatalf("No se pudo conectar con RabbitMQ: %v", err)
	}
	defer connection.Close()

	channel, err := connection.Channel()
	if err != nil {
		log.Fatalf("No se pudo abrir el canal de RabbitMQ: %v", err)
	}
	defer channel.Close()

	queue, err := channel.QueueDeclare(
		queueName,
		true,  // Cola durable.
		false, // No eliminar cuando quede sin consumidores.
		false, // No es exclusiva.
		false, // Esperar la respuesta del servidor.
		nil,
	)
	if err != nil {
		log.Fatalf("No se pudo declarar la cola %s: %v", queueName, err)
	}

	app := &application{
		connection: connection,
		channel:    channel,
		queue:      queue,
	}

	mux := http.NewServeMux()
	mux.HandleFunc("GET /health", app.healthHandler)
	mux.HandleFunc("POST /publish", app.publishHandler)

	server := &http.Server{
		Addr:              ":8084",
		Handler:           mux,
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       10 * time.Second,
		WriteTimeout:      10 * time.Second,
	}

	log.Println("RabbitMQ Writer ejecutándose en http://localhost:8084")
	log.Println("Ruta de salud: GET /health")
	log.Println("Ruta principal: POST /publish")
	log.Printf("Cola configurada: %s", queue.Name)

	if err := server.ListenAndServe(); err != nil &&
		err != http.ErrServerClosed {
		log.Fatalf("Error al ejecutar RabbitMQ Writer: %v", err)
	}
}

func (app *application) healthHandler(
	w http.ResponseWriter,
	_ *http.Request,
) {
	if app.connection.IsClosed() || app.channel.IsClosed() {
		writeJSON(w, http.StatusServiceUnavailable, APIResponse{
			Status:  "error",
			Message: "No existe una conexión activa con RabbitMQ",
		})
		return
	}

	writeJSON(w, http.StatusOK, APIResponse{
		Status:  "ok",
		Message: "RabbitMQ Writer está conectado",
	})
}

func (app *application) publishHandler(
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

	if prediction.HomeTeam == prediction.AwayTeam {
		writeJSON(w, http.StatusBadRequest, APIResponse{
			Status:  "error",
			Message: "Los equipos deben ser diferentes",
		})
		return
	}

	if prediction.HomeGoals < 0 ||
		prediction.HomeGoals > 5 ||
		prediction.AwayGoals < 0 ||
		prediction.AwayGoals > 5 {

		writeJSON(w, http.StatusBadRequest, APIResponse{
			Status:  "error",
			Message: "Los goles deben estar entre 0 y 5",
		})
		return
	}

	messageBody, err := json.Marshal(prediction)
	if err != nil {
		log.Printf("No se pudo convertir la predicción a JSON: %v", err)

		writeJSON(w, http.StatusInternalServerError, APIResponse{
			Status:  "error",
			Message: "No se pudo preparar el mensaje",
		})
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	app.publishMutex.Lock()

	err = app.channel.PublishWithContext(
		ctx,
		"",             // Exchange predeterminado.
		app.queue.Name, // Routing key igual al nombre de la cola.
		false,
		false,
		amqp.Publishing{
			ContentType:  "application/json",
			DeliveryMode: amqp.Persistent,
			Timestamp:    time.Now().UTC(),
			Body:         messageBody,
		},
	)

	app.publishMutex.Unlock()

	if err != nil {
		log.Printf("No se pudo publicar en RabbitMQ: %v", err)

		writeJSON(w, http.StatusBadGateway, APIResponse{
			Status:  "error",
			Message: "No se pudo publicar la predicción en RabbitMQ",
		})
		return
	}

	log.Printf(
		"Predicción publicada en RabbitMQ: %s %d - %d %s | usuario: %s",
		prediction.HomeTeam,
		prediction.HomeGoals,
		prediction.AwayGoals,
		prediction.AwayTeam,
		prediction.Username,
	)

	writeJSON(w, http.StatusOK, APIResponse{
		Status:  "success",
		Message: "Predicción publicada correctamente en RabbitMQ",
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

func getEnvironmentVariable(name string, defaultValue string) string {
	value := os.Getenv(name)

	if value == "" {
		return defaultValue
	}

	return value
}
