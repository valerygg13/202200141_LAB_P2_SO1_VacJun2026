from datetime import datetime, timezone
import random

from locust import HttpUser, between, task


TEAMS = ["GTM", "MEX", "BRA", "ARG", "ESP"]


class QuinielaUser(HttpUser):
    """
    Simula usuarios que envían predicciones de partidos.
    """

    wait_time = between(1, 3)

    @task
    def send_prediction(self) -> None:
        # Selecciona dos equipos diferentes.
        home_team, away_team = random.sample(TEAMS, 2)

        prediction = {
            "home_team": home_team,
            "away_team": away_team,
            "home_goals": random.randint(0, 5),
            "away_goals": random.randint(0, 5),
            "username": f"user_{random.randint(1, 1000)}",
            "timestamp": datetime.now(timezone.utc).isoformat(),
        }

        with self.client.post(
            "/grpc-202200141",
            json=prediction,
            name="/grpc-202200141",
            catch_response=True,
        ) as response:
            if response.status_code == 200:
                response.success()
            else:
                response.failure(
                    f"Error HTTP {response.status_code}: {response.text}"
                )