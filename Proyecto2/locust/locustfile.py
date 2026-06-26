from __future__ import annotations

from datetime import datetime, timezone
import json
import os
import random
from pathlib import Path
from typing import Any

from locust import HttpUser, between, task


PROJECT_ROOT = Path(__file__).resolve().parent.parent

DOWNLOADED_INPUT = (
    PROJECT_ROOT
    / "oci-download-test"
    / "oci-artifacts"
    / "locust-input.json"
)

LOCAL_INPUT = (
    PROJECT_ROOT
    / "oci-artifacts"
    / "locust-input.json"
)


def load_config() -> dict[str, Any]:
    """
    Carga la configuracion descargada desde Zot.

    Se puede indicar otra ruta mediante la variable
    de entorno LOCUST_INPUT_FILE.
    """

    custom_path = os.getenv("LOCUST_INPUT_FILE")

    candidates: list[Path] = []

    if custom_path:
        candidates.append(Path(custom_path).expanduser())

    candidates.extend([
        DOWNLOADED_INPUT,
        LOCAL_INPUT,
    ])

    input_path = next(
        (path for path in candidates if path.is_file()),
        None,
    )

    if input_path is None:
        searched_paths = "\n".join(str(path) for path in candidates)
        raise FileNotFoundError(
            "No se encontro locust-input.json. Rutas revisadas:\n"
            f"{searched_paths}"
        )

    # utf-8-sig permite leer correctamente archivos creados
    # desde Windows PowerShell que incluyan BOM.
    with input_path.open("r", encoding="utf-8-sig") as file:
        config = json.load(file)

    required_fields = {
        "route",
        "assigned_team",
        "allowed_teams",
        "min_goals",
        "max_goals",
    }

    missing_fields = required_fields.difference(config)

    if missing_fields:
        raise ValueError(
            "Faltan campos en locust-input.json: "
            + ", ".join(sorted(missing_fields))
        )

    route = config["route"]
    assigned_team = config["assigned_team"]
    allowed_teams = config["allowed_teams"]
    min_goals = config["min_goals"]
    max_goals = config["max_goals"]

    if not isinstance(route, str) or not route.startswith("/"):
        raise ValueError("route debe ser una ruta que comience con /")

    if not isinstance(allowed_teams, list) or len(allowed_teams) < 2:
        raise ValueError("allowed_teams debe contener al menos dos equipos")

    if assigned_team not in allowed_teams:
        raise ValueError(
            "assigned_team debe estar incluido en allowed_teams"
        )

    if not isinstance(min_goals, int) or not isinstance(max_goals, int):
        raise ValueError("min_goals y max_goals deben ser numeros enteros")

    if min_goals < 0 or min_goals > max_goals:
        raise ValueError("El rango de goles configurado no es valido")

    config["_source_path"] = str(input_path.resolve())

    return config


CONFIG = load_config()

ROUTE = str(CONFIG["route"])
ASSIGNED_TEAM = str(CONFIG["assigned_team"])
ALLOWED_TEAMS = [str(team) for team in CONFIG["allowed_teams"]]
MIN_GOALS = int(CONFIG["min_goals"])
MAX_GOALS = int(CONFIG["max_goals"])

OPPONENTS = [
    team
    for team in ALLOWED_TEAMS
    if team != ASSIGNED_TEAM
]

print(f"[Locust] Configuracion cargada desde: {CONFIG['_source_path']}")
print(f"[Locust] Ruta: {ROUTE}")
print(f"[Locust] Equipo asignado: {ASSIGNED_TEAM}")
print(f"[Locust] Rango de goles: {MIN_GOALS}-{MAX_GOALS}")


class QuinielaUser(HttpUser):
    """
    Simula usuarios que envian predicciones de partidos.
    """

    wait_time = between(1, 3)

    @task
    def send_prediction(self) -> None:
        opponent = random.choice(OPPONENTS)

        # El equipo asignado GTM siempre participa,
        # pero puede jugar como local o visitante.
        if random.choice([True, False]):
            home_team = ASSIGNED_TEAM
            away_team = opponent
        else:
            home_team = opponent
            away_team = ASSIGNED_TEAM

        prediction = {
            "home_team": home_team,
            "away_team": away_team,
            "home_goals": random.randint(MIN_GOALS, MAX_GOALS),
            "away_goals": random.randint(MIN_GOALS, MAX_GOALS),
            "username": f"user_{random.randint(1, 1000)}",
            "timestamp": datetime.now(timezone.utc).isoformat(),
        }

        with self.client.post(
            ROUTE,
            json=prediction,
            name=ROUTE,
            catch_response=True,
        ) as response:
            if response.status_code == 200:
                response.success()
            else:
                response.failure(
                    f"Error HTTP {response.status_code}: {response.text}"
                )
