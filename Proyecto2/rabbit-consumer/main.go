package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"github.com/valkey-io/valkey-go"
	"github.com/valkey-io/valkey-go/valkeycompat"

	amqp "github.com/rabbitmq/amqp091-go"
)

// MatchPrediction representa el mensaje recibido desde RabbitMQ.
type MatchPrediction struct {
	HomeTeam  string `json:"home_team"`
	AwayTeam  string `json:"away_team"`
	HomeGoals int    `json:"home_goals"`
	AwayGoals int    `json:"away_goals"`
	Username  string `json:"username"`
	Timestamp string `json:"timestamp"`
}

// StoredPrediction representa la predicción procesada.
type StoredPrediction struct {
	MatchPrediction
	ProcessedAt string `json:"processed_at"`
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

	valkeyAddress := getEnvironmentVariable(
		"VALKEY_ADDR",
		"localhost:6379",
	)

	ctx, stop := signal.NotifyContext(
		context.Background(),
		os.Interrupt,
		syscall.SIGTERM,
	)
	defer stop()

	// Conexión con Valkey.
	valkeyClient, err := valkey.NewClient(valkey.ClientOption{
		InitAddress: []string{valkeyAddress},
	})
	if err != nil {
		log.Fatalf("No se pudo crear el cliente de Valkey: %v", err)
	}
	defer valkeyClient.Close()

	pingContext, cancelPing := context.WithTimeout(
		context.Background(),
		5*time.Second,
	)

	err = valkeyClient.Do(
		pingContext,
		valkeyClient.B().Ping().Build(),
	).Error()

	cancelPing()

	if err != nil {
		log.Fatalf("No se pudo conectar con Valkey: %v", err)
	}

	// Adaptador de alto nivel del cliente oficial de Valkey.
	database := valkeycompat.NewAdapter(valkeyClient)

	// Conexión con RabbitMQ.
	rabbitConnection, err := amqp.Dial(rabbitMQURL)
	if err != nil {
		log.Fatalf("No se pudo conectar con RabbitMQ: %v", err)
	}
	defer rabbitConnection.Close()

	rabbitChannel, err := rabbitConnection.Channel()
	if err != nil {
		log.Fatalf("No se pudo abrir el canal de RabbitMQ: %v", err)
	}
	defer rabbitChannel.Close()

	queue, err := rabbitChannel.QueueDeclare(
		queueName,
		true,
		false,
		false,
		false,
		nil,
	)
	if err != nil {
		log.Fatalf(
			"No se pudo declarar la cola %s: %v",
			queueName,
			err,
		)
	}

	// Máximo de mensajes pendientes por consumidor.
	if err := rabbitChannel.Qos(20, 0, false); err != nil {
		log.Fatalf("No se pudo configurar QoS: %v", err)
	}

	deliveries, err := rabbitChannel.Consume(
		queue.Name,
		"quiniela-consumer",
		false, // Confirmación manual.
		false,
		false,
		false,
		nil,
	)
	if err != nil {
		log.Fatalf("No se pudo iniciar el consumidor: %v", err)
	}

	log.Println("RabbitMQ Consumer iniciado")
	log.Printf("Cola: %s", queue.Name)
	log.Printf("Valkey: %s", valkeyAddress)
	log.Println("Esperando predicciones...")

	for {
		select {
		case <-ctx.Done():
			log.Println("Deteniendo RabbitMQ Consumer")
			return

		case delivery, open := <-deliveries:
			if !open {
				log.Println("El canal de RabbitMQ fue cerrado")
				return
			}

			processDelivery(database, delivery)
		}
	}
}

func processDelivery(
	database valkeycompat.Cmdable,
	delivery amqp.Delivery,
) {
	var prediction MatchPrediction

	if err := json.Unmarshal(delivery.Body, &prediction); err != nil {
		log.Printf("Mensaje JSON inválido: %v", err)

		// No volver a colocar mensajes inválidos en la cola.
		if nackErr := delivery.Nack(false, false); nackErr != nil {
			log.Printf("No se pudo rechazar el mensaje: %v", nackErr)
		}

		return
	}

	if err := validatePrediction(prediction); err != nil {
		log.Printf("Predicción inválida: %v", err)

		if nackErr := delivery.Nack(false, false); nackErr != nil {
			log.Printf("No se pudo rechazar la predicción: %v", nackErr)
		}

		return
	}

	storeContext, cancel := context.WithTimeout(
		context.Background(),
		5*time.Second,
	)
	defer cancel()

	if err := storePrediction(
		storeContext,
		database,
		prediction,
	); err != nil {
		log.Printf("No se pudo guardar en Valkey: %v", err)

		// Volver a colocar el mensaje en RabbitMQ.
		if nackErr := delivery.Nack(false, true); nackErr != nil {
			log.Printf("No se pudo reencolar el mensaje: %v", nackErr)
		}

		return
	}

	if err := delivery.Ack(false); err != nil {
		log.Printf("No se pudo confirmar el mensaje: %v", err)
		return
	}

	log.Printf(
		"Predicción almacenada en Valkey: %s %d - %d %s | usuario: %s",
		prediction.HomeTeam,
		prediction.HomeGoals,
		prediction.AwayGoals,
		prediction.AwayTeam,
		prediction.Username,
	)
}

func storePrediction(
	ctx context.Context,
	database valkeycompat.Cmdable,
	prediction MatchPrediction,
) error {
	storedPrediction := StoredPrediction{
		MatchPrediction: prediction,
		ProcessedAt:     time.Now().UTC().Format(time.RFC3339Nano),
	}

	jsonBody, err := json.Marshal(storedPrediction)
	if err != nil {
		return fmt.Errorf(
			"no se pudo convertir la predicción a JSON: %w",
			err,
		)
	}

	homeTimePoint, err := json.Marshal(map[string]interface{}{
		"timestamp": prediction.Timestamp,
		"goals":     prediction.HomeGoals,
		"username":  prediction.Username,
	})
	if err != nil {
		return fmt.Errorf(
			"no se pudo crear la serie local: %w",
			err,
		)
	}

	awayTimePoint, err := json.Marshal(map[string]interface{}{
		"timestamp": prediction.Timestamp,
		"goals":     prediction.AwayGoals,
		"username":  prediction.Username,
	})
	if err != nil {
		return fmt.Errorf(
			"no se pudo crear la serie visitante: %w",
			err,
		)
	}

	_, err = database.Pipelined(
		ctx,
		func(pipe valkeycompat.Pipeliner) error {
			// Guardar todas las predicciones.
			pipe.RPush(
				ctx,
				"predictions:raw",
				string(jsonBody),
			)

			// Cantidad total de predicciones.
			pipe.Incr(ctx, "stats:total")

			// Cantidad de predicciones por usuario.
			pipe.ZIncrBy(
				ctx,
				"stats:users",
				1,
				prediction.Username,
			)

			// Frecuencia de goles locales y visitantes.
			pipe.ZIncrBy(
				ctx,
				"stats:home_goals_frequency",
				1,
				strconv.Itoa(prediction.HomeGoals),
			)

			pipe.ZIncrBy(
				ctx,
				"stats:away_goals_frequency",
				1,
				strconv.Itoa(prediction.AwayGoals),
			)

			// Cantidad de predicciones donde aparece cada equipo.
			pipe.Incr(
				ctx,
				"stats:team:"+prediction.HomeTeam+":total",
			)

			pipe.Incr(
				ctx,
				"stats:team:"+prediction.AwayTeam+":total",
			)

			// Series temporales para cada equipo.
			pipe.RPush(
				ctx,
				"timeseries:"+prediction.HomeTeam+":home",
				string(homeTimePoint),
			)

			pipe.RPush(
				ctx,
				"timeseries:"+prediction.AwayTeam+":away",
				string(awayTimePoint),
			)

			// Equipo ganador predicho.
			if prediction.HomeGoals > prediction.AwayGoals {
				pipe.ZIncrBy(
					ctx,
					"stats:team_wins",
					1,
					prediction.HomeTeam,
				)
			}

			if prediction.AwayGoals > prediction.HomeGoals {
				pipe.ZIncrBy(
					ctx,
					"stats:team_wins",
					1,
					prediction.AwayTeam,
				)
			}

			return nil
		},
	)
	if err != nil {
		return fmt.Errorf(
			"no se pudieron almacenar las métricas: %w",
			err,
		)
	}

	if err := updateMaximum(
		ctx,
		database,
		"stats:max_home_goals",
		prediction.HomeGoals,
	); err != nil {
		return err
	}

	if err := updateMinimum(
		ctx,
		database,
		"stats:min_home_goals",
		prediction.HomeGoals,
	); err != nil {
		return err
	}

	if err := updateMaximum(
		ctx,
		database,
		"stats:max_away_goals",
		prediction.AwayGoals,
	); err != nil {
		return err
	}

	if err := updateMinimum(
		ctx,
		database,
		"stats:min_away_goals",
		prediction.AwayGoals,
	); err != nil {
		return err
	}

	return nil
}

func updateMaximum(
	ctx context.Context,
	database valkeycompat.Cmdable,
	key string,
	value int,
) error {
	currentValue, err := database.Get(ctx, key).Int()

	if err == valkeycompat.Nil {
		return database.Set(ctx, key, value, 0).Err()
	}

	if err != nil {
		return fmt.Errorf("no se pudo consultar %s: %w", key, err)
	}

	if value > currentValue {
		if err := database.Set(ctx, key, value, 0).Err(); err != nil {
			return fmt.Errorf("no se pudo actualizar %s: %w", key, err)
		}
	}

	return nil
}

func updateMinimum(
	ctx context.Context,
	database valkeycompat.Cmdable,
	key string,
	value int,
) error {
	currentValue, err := database.Get(ctx, key).Int()

	if err == valkeycompat.Nil {
		return database.Set(ctx, key, value, 0).Err()
	}

	if err != nil {
		return fmt.Errorf("no se pudo consultar %s: %w", key, err)
	}

	if value < currentValue {
		if err := database.Set(ctx, key, value, 0).Err(); err != nil {
			return fmt.Errorf("no se pudo actualizar %s: %w", key, err)
		}
	}

	return nil
}

func validatePrediction(prediction MatchPrediction) error {
	if prediction.HomeTeam == "" ||
		prediction.AwayTeam == "" ||
		prediction.Username == "" ||
		prediction.Timestamp == "" {
		return fmt.Errorf("faltan campos obligatorios")
	}

	if prediction.HomeTeam == prediction.AwayTeam {
		return fmt.Errorf("los equipos deben ser diferentes")
	}

	if prediction.HomeGoals < 0 ||
		prediction.HomeGoals > 5 ||
		prediction.AwayGoals < 0 ||
		prediction.AwayGoals > 5 {
		return fmt.Errorf("los goles deben estar entre 0 y 5")
	}

	return nil
}

func getEnvironmentVariable(
	name string,
	defaultValue string,
) string {
	value := os.Getenv(name)

	if value == "" {
		return defaultValue
	}

	return value
}
