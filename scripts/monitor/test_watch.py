import tempfile
import unittest
from pathlib import Path

import watch


def settings(**overrides):
    values = dict(
        email_to="ops@example.com",
        email_from="ops@example.com",
        smtp_host="smtp.example.com",
        smtp_port=587,
        smtp_user="ops@example.com",
        smtp_password="secret",
        smtp_tls="starttls",
        http_url="http://127.0.0.1:8081/server/ok",
        postgres_container="evolution-go-postgres",
        postgres_user="evogo",
        postgres_db="evogo_users",
        state_file="/tmp/evolution-go-monitor-state.json",
        api_after=120,
        session_after=600,
        repeat_after=21600,
        drop_ratio=0.5,
        recovery_streak=2,
    )
    values.update(overrides)
    return watch.Settings(**values)


def snap(**overrides):
    values = dict(
        http_ok=True, http_error="", paired=32, connected=24, db_error="",
        http_url="http://127.0.0.1:8081/server/ok",
    )
    values.update(overrides)
    return watch.Snapshot(**values)


class WatchDecisionTest(unittest.TestCase):
    def test_api_down_waits_for_the_persistence_window(self):
        start = watch.empty_episode(1_000)
        episode, notice = watch.advance(start, snap(http_ok=False, http_error="refused"), 1_000, settings())
        self.assertEqual(episode.status, watch.API_DOWN)
        self.assertIsNone(notice)
        episode, notice = watch.advance(episode, snap(http_ok=False, http_error="refused"), 1_119, settings())
        self.assertIsNone(notice)
        episode, notice = watch.advance(episode, snap(http_ok=False, http_error="refused"), 1_120, settings())
        self.assertEqual(notice[0], "[Evolution Go] API sem resposta")
        self.assertTrue(episode.alerted)

    def test_api_down_repeats_only_after_the_cooldown(self):
        start = watch.Episode(watch.API_DOWN, 1_000, 0, True, 1_120, 24)
        _, notice = watch.advance(start, snap(http_ok=False, http_error="refused"), 1_120 + 21599, settings())
        self.assertIsNone(notice)
        _, notice = watch.advance(start, snap(http_ok=False, http_error="refused"), 1_120 + 21600, settings())
        self.assertEqual(notice[0], "[Evolution Go] API sem resposta")

    def test_all_paired_sessions_down_alerts_after_ten_minutes(self):
        start = watch.empty_episode(5_000)
        episode, notice = watch.advance(start, snap(connected=0), 5_000, settings())
        self.assertEqual(episode.status, watch.SESSIONS_DOWN)
        self.assertIsNone(notice)
        _, notice = watch.advance(episode, snap(connected=0), 5_599, settings())
        self.assertIsNone(notice)
        _, notice = watch.advance(episode, snap(connected=0), 5_600, settings())
        self.assertEqual(notice[0], "[Evolution Go] sessões WhatsApp desconectadas")
        self.assertIn("Pareadas: 32", notice[1])

    def test_losing_half_or_more_sessions_alerts(self):
        start = watch.Episode(watch.OK, 1, 2, False, None, 24)
        _, quiet = watch.advance(start, snap(connected=13), 100, settings())
        self.assertIsNone(quiet)
        episode, waiting = watch.advance(start, snap(connected=12), 100, settings())
        self.assertEqual(episode.status, watch.SESSIONS_DEGRADED)
        self.assertIsNone(waiting)
        _, notice = watch.advance(episode, snap(connected=12), 700, settings())
        self.assertIn("Última leitura estável: 24", notice[1])

    def test_one_disconnected_session_does_not_lower_the_baseline(self):
        start = watch.Episode(watch.OK, 1, 2, False, None, 24)
        episode, notice = watch.advance(start, snap(connected=23), 800, settings())
        self.assertEqual(episode.status, watch.OK)
        self.assertIsNone(notice)
        self.assertEqual(episode.baseline_connected, 24)
        grown, _ = watch.advance(episode, snap(connected=25), 900, settings())
        self.assertEqual(grown.baseline_connected, 25)

    def test_no_paired_instances_stays_quiet(self):
        start = watch.empty_episode(1)
        episode, notice = watch.advance(start, snap(paired=0, connected=0), 10_000, settings())
        self.assertEqual(episode.status, watch.OK)
        self.assertIsNone(notice)

    def test_api_failure_takes_priority_over_session_counts(self):
        start = watch.Episode(watch.OK, 1, 2, False, None, 24)
        episode, _ = watch.advance(start, snap(http_ok=False, http_error="timeout", connected=0), 200, settings())
        self.assertEqual(episode.status, watch.API_DOWN)

    def test_recovery_waits_for_two_healthy_samples_and_resets_baseline(self):
        start = watch.Episode(watch.OK, 1, 2, False, None, 24)
        fallen, _ = watch.advance(start, snap(connected=0), 100, settings())
        down, notice = watch.advance(fallen, snap(connected=0), 700, settings())
        self.assertEqual(notice[0], "[Evolution Go] sessões WhatsApp desconectadas")
        holding, notice = watch.advance(down, snap(connected=20), 800, settings())
        self.assertIsNone(notice)
        self.assertTrue(holding.alerted)
        self.assertEqual(holding.baseline_connected, 24)
        restored, notice = watch.advance(holding, snap(connected=20), 900, settings())
        self.assertEqual(notice[0], "[Evolution Go] serviço restabelecido")
        self.assertIn("sessões desconectadas", notice[1])
        self.assertFalse(restored.alerted)
        self.assertEqual(restored.baseline_connected, 20)

    def test_a_new_failure_can_alert_after_another_failure(self):
        api = watch.Episode(watch.API_DOWN, 1, 0, True, 200, 24)
        changed, notice = watch.advance(api, snap(connected=0), 300, settings())
        self.assertEqual(changed.status, watch.SESSIONS_DOWN)
        self.assertIsNone(notice)
        self.assertFalse(changed.alerted)
        _, notice = watch.advance(changed, snap(connected=0), 900, settings())
        self.assertEqual(notice[0], "[Evolution Go] sessões WhatsApp desconectadas")

    def test_database_read_failure_alerts_without_calling_sessions_healthy(self):
        start = watch.empty_episode(1)
        episode, waiting = watch.advance(
            start, snap(paired=None, connected=None, db_error="psql exited 2"), 1, settings(),
        )
        self.assertEqual(episode.status, watch.CHECK_FAILED)
        self.assertIsNone(waiting)
        _, notice = watch.advance(
            episode, snap(paired=None, connected=None, db_error="psql exited 2"), 700, settings(),
        )
        self.assertIn("psql exited 2", notice[1])

    def test_unsent_notice_is_not_remembered_as_delivered(self):
        with tempfile.TemporaryDirectory() as directory:
            config = settings(smtp_host="", state_file=str(Path(directory) / "state.json"))
            _, notice = watch.run(
                config, now=1_000, snapshot=snap(http_ok=False, http_error="refused"),
            )
            self.assertIsNone(notice)
            episode, notice = watch.run(
                config, now=1_200, snapshot=snap(http_ok=False, http_error="refused"),
            )
            self.assertIsNotNone(notice)
            self.assertFalse(episode.alerted)
            saved = watch.load_episode(config.state_file, 1_200)
            self.assertFalse(saved.alerted)


if __name__ == "__main__":
    unittest.main()
