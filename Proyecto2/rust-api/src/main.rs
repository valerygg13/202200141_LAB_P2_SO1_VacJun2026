use axum::{
    Json, Router,
    http::StatusCode,
    response::IntoResponse,
    routing::{get, post},
};
use serde::{Deserialize, Serialize};

const VALID_TEAMS: [&str; 5] = ["GTM", "MEX", "BRA", "ARG", "ESP"];

#[derive(Debug, Deserialize)]
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
    let app = Router::new()
        .route("/health", get(health))
        .route("/grpc-202200141", post(receive_prediction));

    let address = "0.0.0.0:8081";

    let listener = tokio::net::TcpListener::bind(address)
        .await
        .expect("No se pudo abrir el puerto 8081");

    println!("API Rust ejecutándose en http://localhost:8081");
    println!("Ruta de salud: GET /health");
    println!("Ruta principal: POST /grpc-202200141");

    axum::serve(listener, app)
        .await
        .expect("Error al ejecutar el servidor");
}

async fn health() -> impl IntoResponse {
    let response = ApiResponse {
        status: "ok".to_string(),
        message: "La API Rust está funcionando".to_string(),
    };

    (StatusCode::OK, Json(response))
}

async fn receive_prediction(Json(prediction): Json<MatchPrediction>) -> impl IntoResponse {
    if !VALID_TEAMS.contains(&prediction.home_team.as_str())
        || !VALID_TEAMS.contains(&prediction.away_team.as_str())
    {
        let response = ApiResponse {
            status: "error".to_string(),
            message: "El equipo local o visitante no es válido".to_string(),
        };

        return (StatusCode::BAD_REQUEST, Json(response));
    }

    if prediction.home_team == prediction.away_team {
        let response = ApiResponse {
            status: "error".to_string(),
            message: "El equipo local y visitante deben ser diferentes".to_string(),
        };

        return (StatusCode::BAD_REQUEST, Json(response));
    }

    if prediction.home_goals > 5 || prediction.away_goals > 5 {
        let response = ApiResponse {
            status: "error".to_string(),
            message: "Los goles deben estar entre 0 y 5".to_string(),
        };

        return (StatusCode::BAD_REQUEST, Json(response));
    }

    if prediction.username.trim().is_empty() || prediction.timestamp.trim().is_empty() {
        let response = ApiResponse {
            status: "error".to_string(),
            message: "El usuario y timestamp son obligatorios".to_string(),
        };

        return (StatusCode::BAD_REQUEST, Json(response));
    }

    println!("Predicción recibida:");
    println!("{:#?}", prediction);

    let response = ApiResponse {
        status: "success".to_string(),
        message: "Predicción recibida correctamente".to_string(),
    };

    (StatusCode::OK, Json(response))
}
