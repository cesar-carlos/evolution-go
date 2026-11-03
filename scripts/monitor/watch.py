"""Email when Evolution Go or its WhatsApp sessions stay down.

The check runs outside the API process: a dead container cannot send its own
alert, and /server/ok stayed healthy during the session outage this watches for.
"""

import json
import os
import smtplib
import ssl
import subprocess
import time
import urllib.error
import urllib.request
from dataclasses import asdict, dataclass
from email.message import EmailMessage
from pathlib import Path
import sys

OK = "ok"
API_DOWN = "api_down"
SESSIONS_DOWN = "sessions_down"
SESSIONS_DEGRADED = "sessions_degraded"
CHECK_FAILED = "check_failed"

DEFAULT_ENV_FILE = "/etc/evolution-go-monitor.env"
DEFAULT_STATE_FILE = "/var/lib/evolution-go-monitor/state.json"
SESSION_SQL = (
    "SELECT count(*) FILTER (WHERE btrim(coalesce(jid, '')) <> ''), "
    "count(*) FILTER (WHERE btrim(coalesce(jid, '')) <> '' AND connected) "
    "FROM instances"
)


@dataclass
class Settings:
    email_to: str
    email_from: str
    smtp_host: str
    smtp_port: int
    smtp_user: str
    smtp_password: str
    smtp_tls: str
    http_url: str
    postgres_container: str
    postgres_user: str
    postgres_db: str
    state_file: str
    api_after: float
    session_after: float
    repeat_after: float
    drop_ratio: float
    recovery_streak: int


@dataclass
class Snapshot:
    http_ok: bool
    http_error: str
    paired: int | None
    connected: int | None
    db_error: str
    http_url: str


@dataclass
class Episode:
    status: str
    since: float
    ok_streak: int
    alerted: bool
    last_alert_at: float | None
    baseline_connected: int | None
    alert_status: str | None = None


def load_env_file(path):
    """Fill missing process environment keys from a KEY=VALUE file."""
    file = Path(path)
    if not file.is_file():
        return
    for raw in file.read_text(encoding="utf-8").splitlines():
        line = raw.strip()
        if not line or line.startswith("#") or "=" not in line:
            continue
        key, value = line.split("=", 1)
        os.environ.setdefault(key.strip(), value.strip().strip("'\""))


def _required_number(name, default, kind):
    raw = os.environ.get(name, "").strip()
    if raw == "":
        return kind(default)
    try:
        return kind(raw)
    except ValueError as exc:
        raise ValueError(f"{name} must be a number") from exc


def load_settings():
    tls = os.environ.get("MONITOR_SMTP_TLS", "starttls").strip().lower() or "starttls"
    if tls not in {"starttls", "ssl", "none"}:
        raise ValueError("MONITOR_SMTP_TLS must be starttls, ssl, or none")
    drop_ratio = _required_number("MONITOR_DROP_RATIO", 0.5, float)
    if not 0 < drop_ratio < 1:
        raise ValueError("MONITOR_DROP_RATIO must be between 0 and 1")
    recovery = _required_number("MONITOR_RECOVERY_STREAK", 2, int)
    if recovery < 1:
        raise ValueError("MONITOR_RECOVERY_STREAK must be at least 1")
    email_to = os.environ.get("MONITOR_EMAIL_TO", "").strip()
    return Settings(
        email_to=email_to,
        email_from=os.environ.get("MONITOR_EMAIL_FROM", "").strip() or email_to,
        smtp_host=os.environ.get("MONITOR_SMTP_HOST", "").strip(),
        smtp_port=_required_number("MONITOR_SMTP_PORT", 587, int),
        smtp_user=os.environ.get("MONITOR_SMTP_USER", "").strip(),
        smtp_password=os.environ.get("MONITOR_SMTP_PASSWORD", ""),
        smtp_tls=tls,
        http_url=os.environ.get("MONITOR_HTTP_URL", "http://127.0.0.1:8081/server/ok").strip(),
        postgres_container=os.environ.get("MONITOR_POSTGRES_CONTAINER", "evolution-go-postgres").strip(),
        postgres_user=os.environ.get("MONITOR_POSTGRES_USER", "evogo").strip(),
        postgres_db=os.environ.get("MONITOR_POSTGRES_DB", "evogo_users").strip(),
        state_file=os.environ.get("MONITOR_STATE_FILE", DEFAULT_STATE_FILE).strip(),
        api_after=_required_number("MONITOR_API_DOWN_SECONDS", 120, float),
        session_after=_required_number("MONITOR_SESSION_DOWN_SECONDS", 600, float),
        repeat_after=_required_number("MONITOR_REPEAT_SECONDS", 21600, float),
        drop_ratio=drop_ratio,
        recovery_streak=recovery,
    )


def empty_episode(now):
    return Episode(OK, now, 0, False, None, None, None)


def load_episode(path, now):
    file = Path(path)
    if not file.is_file():
        return empty_episode(now)
    try:
        data = json.loads(file.read_text(encoding="utf-8"))
        return Episode(
            status=str(data.get("status") or OK),
            since=float(data.get("since", now)),
            ok_streak=int(data.get("ok_streak", 0)),
            alerted=bool(data.get("alerted", False)),
            last_alert_at=data.get("last_alert_at"),
            baseline_connected=data.get("baseline_connected"),
            alert_status=data.get("alert_status"),
        )
    except (OSError, json.JSONDecodeError, TypeError, ValueError):
        return empty_episode(now)


def save_episode(path, episode):
    file = Path(path)
    file.parent.mkdir(parents=True, exist_ok=True)
    temporary = file.with_suffix(file.suffix + ".tmp")
    temporary.write_text(json.dumps(asdict(episode)), encoding="utf-8")
    os.replace(temporary, file)


def check_http(url, timeout=5):
    request = urllib.request.Request(url)
    try:
        with urllib.request.urlopen(request, timeout=timeout) as response:
            response.read(256)
            if response.status != 200:
                return False, f"HTTP {response.status}"
            return True, ""
    except urllib.error.HTTPError as exc:
        return False, f"HTTP {exc.code}"
    except Exception as exc:  # noqa: BLE001 - the alert must report any probe failure
        return False, str(exc)


def check_sessions(container, user, database, timeout=20):
    command = [
        "docker", "exec", container, "psql",
        "-U", user, "-d", database, "-tAc", SESSION_SQL,
    ]
    try:
        completed = subprocess.run(
            command, check=False, capture_output=True, text=True, timeout=timeout,
        )
    except (OSError, subprocess.TimeoutExpired) as exc:
        return None, None, str(exc)
    if completed.returncode != 0:
        detail = (completed.stderr or completed.stdout).strip()
        return None, None, detail or f"psql exited {completed.returncode}"
    parts = completed.stdout.strip().split("|")
    if len(parts) != 2:
        return None, None, "unexpected session count result"
    try:
        return int(parts[0]), int(parts[1]), ""
    except ValueError:
        return None, None, "unexpected session count result"


def classify(snapshot, baseline_connected, drop_ratio):
    if not snapshot.http_ok:
        return API_DOWN
    if snapshot.db_error or snapshot.paired is None or snapshot.connected is None:
        return CHECK_FAILED
    above_floor = baseline_connected is None or snapshot.connected > baseline_connected * (1 - drop_ratio)
    if snapshot.paired == 0 or (snapshot.connected > 0 and above_floor):
        return OK
    if snapshot.connected == 0:
        return SESSIONS_DOWN
    return SESSIONS_DEGRADED


def _stamp(now):
    return time.strftime("%Y-%m-%d %H:%M:%S UTC", time.gmtime(now))


def incident_notice(status, snapshot, baseline, now):
    if status == API_DOWN:
        subject = "[Evolution Go] API sem resposta"
        detail = f"A API não respondeu.\nURL: {snapshot.http_url}\nErro: {snapshot.http_error}\n"
    elif status == SESSIONS_DOWN:
        subject = "[Evolution Go] sessões WhatsApp desconectadas"
        detail = (
            "A API responde, mas nenhuma instância pareada está conectada.\n"
            f"Pareadas: {snapshot.paired}\nConectadas: {snapshot.connected}\n"
        )
    elif status == SESSIONS_DEGRADED:
        subject = "[Evolution Go] queda de sessões WhatsApp"
        detail = (
            "As sessões conectadas caíram à metade ou menos da última leitura estável.\n"
            f"Conectadas agora: {snapshot.connected}\n"
            f"Última leitura estável: {baseline}\n"
            f"Pareadas: {snapshot.paired}\n"
        )
    else:
        subject = "[Evolution Go] falha ao verificar as sessões"
        detail = f"A API responde, mas a leitura das instâncias falhou.\nErro: {snapshot.db_error}\n"
    return subject, detail + f"Horário: {_stamp(now)}\n"


def recovery_notice(alert_status, snapshot, now):
    labels = {
        API_DOWN: "API sem resposta",
        SESSIONS_DOWN: "sessões desconectadas",
        SESSIONS_DEGRADED: "queda de sessões",
        CHECK_FAILED: "falha na verificação",
    }
    subject = "[Evolution Go] serviço restabelecido"
    body = (
        f"A interrupção ({labels.get(alert_status, alert_status)}) encerrou.\n"
        f"Pareadas: {snapshot.paired}\nConectadas: {snapshot.connected}\n"
        f"Horário: {_stamp(now)}\n"
    )
    return subject, body


def advance(previous, snapshot, now, settings):
    """Move the episode forward and return a notice once the outage persists."""
    status = classify(snapshot, previous.baseline_connected, settings.drop_ratio)
    changed = status != previous.status
    since = now if changed else previous.since
    ok_streak = previous.ok_streak + 1 if status == OK else 0
    if changed and status != OK:
        alerted = False
    else:
        alerted = previous.alerted
    last_alert_at = previous.last_alert_at
    baseline = previous.baseline_connected
    alert_status = None if changed and status != OK else previous.alert_status
    notice = None

    if status == OK:
        if previous.alerted and ok_streak >= settings.recovery_streak:
            notice = recovery_notice(previous.alert_status or previous.status, snapshot, now)
            alerted = False
            alert_status = None
            last_alert_at = now
            if snapshot.connected is not None:
                baseline = snapshot.connected
        elif not previous.alerted and snapshot.connected is not None and snapshot.connected > (baseline or 0):
            baseline = snapshot.connected
    else:
        elapsed = now - since
        needed = settings.api_after if status == API_DOWN else settings.session_after
        repeat = alerted and last_alert_at is not None and now - last_alert_at >= settings.repeat_after
        if elapsed >= needed and (not alerted or repeat):
            notice = incident_notice(status, snapshot, baseline, now)
            alerted = True
            alert_status = status
            last_alert_at = now

    return Episode(status, since, ok_streak, alerted, last_alert_at, baseline, alert_status), notice


def can_send(settings):
    return bool(settings.smtp_host and settings.email_to and settings.email_from)


def send_email(settings, subject, body, timeout=30):
    message = EmailMessage()
    message["Subject"] = subject
    message["From"] = settings.email_from
    message["To"] = settings.email_to
    message.set_content(body)
    context = ssl.create_default_context()
    if settings.smtp_tls == "ssl":
        smtp = smtplib.SMTP_SSL(settings.smtp_host, settings.smtp_port, timeout=timeout, context=context)
    else:
        smtp = smtplib.SMTP(settings.smtp_host, settings.smtp_port, timeout=timeout)
    try:
        smtp.ehlo()
        if settings.smtp_tls == "starttls":
            smtp.starttls(context=context)
            smtp.ehlo()
        if settings.smtp_user:
            smtp.login(settings.smtp_user, settings.smtp_password)
        smtp.send_message(message)
    finally:
        try:
            smtp.quit()
        except Exception:
            smtp.close()


def probe(settings):
    http_ok, http_error = check_http(settings.http_url)
    paired, connected, db_error = check_sessions(
        settings.postgres_container, settings.postgres_user, settings.postgres_db,
    )
    return Snapshot(http_ok, http_error, paired, connected, db_error, settings.http_url)


def run(settings, now=None, snapshot=None):
    moment = time.time() if now is None else now
    current = probe(settings) if snapshot is None else snapshot
    previous = load_episode(settings.state_file, moment)
    episode, notice = advance(previous, current, moment, settings)
    if notice is None:
        save_episode(settings.state_file, episode)
        return episode, None
    if not can_send(settings):
        episode.alerted = previous.alerted
        episode.last_alert_at = previous.last_alert_at
        save_episode(settings.state_file, episode)
        print("alerta não enviado; configure MONITOR_SMTP_HOST e MONITOR_EMAIL_TO", file=sys.stderr)
        return episode, notice
    try:
        send_email(settings, notice[0], notice[1])
    except Exception:
        episode.alerted = previous.alerted
        episode.last_alert_at = previous.last_alert_at
        save_episode(settings.state_file, episode)
        raise
    save_episode(settings.state_file, episode)
    print(notice[0])
    return episode, notice


def main():
    env_file = os.environ.get("MONITOR_ENV_FILE", DEFAULT_ENV_FILE)
    load_env_file(env_file)
    settings = load_settings()
    run(settings)


if __name__ == "__main__":
    main()
