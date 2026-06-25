use std::env;

use axum::{
    Json, Router,
    extract::State,
    http::StatusCode,
    response::IntoResponse,
    routing::{get, post},
};
use serde::{Deserialize, Serialize};

const VALID_TEAMS: [&str; 5] = ["GTM", "MEX", "BRA", "ARG", "ESP"];

#[derive(Clone)]
struct AppState {
    http_client: reqwest::Client,
    go_api_url: String,
}

#[derive(Debug, Deserialize, Serialize)]
struct MatchPrediction {
    home_team: String,
    away_team: String,
    home_goals: u8,
    away_goals: u8,
    username: String,
    timestamp: String,
}

#[derive(Debug, Serialize)]
struct ApiResponse {
    status: String,
    message: String,
}

#[tokio::main]
async fn main() {
    // Por ahora apunta a la API Go ejecutada localmente.
    // En Kubernetes cambiaremos este valor mediante una variable de entorno.
    let go_api_url =
        env::var("GO_API_URL").unwrap_or_else(|_| "http://localhost:8082/prediction".to_string());

    let state = AppState {
        http_client: reqwest::Client::new(),
        go_api_url,
    };

    let app = Router::new()
        .route("/health", get(health))
        .route("/grpc-202200141", post(receive_prediction))
        .with_state(state.clone());

    let address = "0.0.0.0:8081";

    let listener = tokio::net::TcpListener::bind(address)
        .await
        .expect("No se pudo abrir el puerto 8081");

    println!("API Rust ejecutándose en http://localhost:8081");
    println!("Ruta de salud: GET /health");
    println!("Ruta principal: POST /grpc-202200141");
    println!("Destino de las predicciones: {}", state.go_api_url);

    axum::serve(listener, app)
        .await
        .expect("Error al ejecutar el servidor");
}

async fn health() -> impl IntoResponse {
    create_response(StatusCode::OK, "ok", "La API Rust está funcionando")
}

async fn receive_prediction(
    State(state): State<AppState>,
    Json(prediction): Json<MatchPrediction>,
) -> impl IntoResponse {
    if !VALID_TEAMS.contains(&prediction.home_team.as_str())
        || !VALID_TEAMS.contains(&prediction.away_team.as_str())
    {
        return create_response(
            StatusCode::BAD_REQUEST,
            "error",
            "El equipo local o visitante no es válido",
        );
    }

    if prediction.home_team == prediction.away_team {
        return create_response(
            StatusCode::BAD_REQUEST,
            "error",
            "El equipo local y visitante deben ser diferentes",
        );
    }

    if prediction.home_goals > 5 || prediction.away_goals > 5 {
        return create_response(
            StatusCode::BAD_REQUEST,
            "error",
            "Los goles deben estar entre 0 y 5",
        );
    }

    if prediction.username.trim().is_empty() || prediction.timestamp.trim().is_empty() {
        return create_response(
            StatusCode::BAD_REQUEST,
            "error",
            "El usuario y timestamp son obligatorios",
        );
    }

    println!("Predicción válida recibida en Rust:");
    println!("{prediction:#?}");

    let result = state
        .http_client
        .post(&state.go_api_url)
        .json(&prediction)
        .send()
        .await;

    match result {
        Ok(response) if response.status().is_success() => {
            println!("Predicción enviada correctamente a Go");

            create_response(
                StatusCode::OK,
                "success",
                "Predicción recibida por Rust y enviada correctamente a Go",
            )
        }

        Ok(response) => {
            eprintln!(
                "La API Go rechazó la petición. Estado: {}",
                response.status()
            );

            create_response(
                StatusCode::BAD_GATEWAY,
                "error",
                "La API Go rechazó la predicción",
            )
        }

        Err(error) => {
            eprintln!("No se pudo conectar con la API Go: {error}");

            create_response(
                StatusCode::BAD_GATEWAY,
                "error",
                "No se pudo conectar con la API Go",
            )
        }
    }
}

fn create_response(
    status_code: StatusCode,
    status: &str,
    message: &str,
) -> (StatusCode, Json<ApiResponse>) {
    let response = ApiResponse {
        status: status.to_string(),
        message: message.to_string(),
    };

    (status_code, Json(response))
}
