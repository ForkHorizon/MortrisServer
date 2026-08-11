#!/usr/bin/env python3
"""
Polling-only Telegram admin shell bot.

Install:
    python3 -m pip install "python-telegram-bot>=20"

Run:
    python3 tg_admin_shell_bot.py

Local checks:
    python3 -m py_compile tg_admin_shell_bot.py
    python3 tg_admin_shell_bot.py --help

Only Telegram user id 160098235 is authorized. Other users are ignored.
"""

from __future__ import annotations

import argparse
import asyncio
import mimetypes
import os
import shlex
import signal
import sys
from dataclasses import dataclass
from pathlib import Path
from typing import Awaitable, Callable, Iterable

try:
    from telegram import Update
    from telegram.constants import ChatAction
    from telegram.ext import (
        Application,
        CommandHandler,
        ContextTypes,
        MessageHandler,
        filters,
    )
except ImportError as exc:  # pragma: no cover - exercised only when dependency is missing.
    Update = None  # type: ignore[assignment]
    ChatAction = None  # type: ignore[assignment]
    Application = None  # type: ignore[assignment]
    CommandHandler = None  # type: ignore[assignment]
    ContextTypes = None  # type: ignore[assignment]
    MessageHandler = None  # type: ignore[assignment]
    filters = None  # type: ignore[assignment]
    TELEGRAM_IMPORT_ERROR = exc
else:
    TELEGRAM_IMPORT_ERROR = None


BOT_TOKEN = "1046433797:AAF6AG702wzhXqxt4g5sQY3DeyQTEtNzVDU"
ADMIN_USER_ID = 160098235
COMMAND_TIMEOUT_SECONDS = 60
TELEGRAM_TEXT_LIMIT = 4096
MESSAGE_CHUNK_SIZE = 3800


@dataclass
class ShellState:
    cwd: Path


STATE = ShellState(cwd=Path.home().resolve())


def is_admin(update: Update) -> bool:
    user = update.effective_user
    return bool(user and user.id == ADMIN_USER_ID)


def split_text(text: str, chunk_size: int = MESSAGE_CHUNK_SIZE) -> Iterable[str]:
    if not text:
        yield "(no output)"
        return

    for start in range(0, len(text), chunk_size):
        yield text[start : start + chunk_size]


async def send_text_chunks(update: Update, text: str) -> None:
    message = update.effective_message
    if not message:
        return

    for chunk in split_text(text):
        if len(chunk) > TELEGRAM_TEXT_LIMIT:
            chunk = chunk[: TELEGRAM_TEXT_LIMIT - 20] + "\n...[truncated chunk]"
        await message.reply_text(chunk)


def resolve_cd_path(raw_path: str) -> Path:
    if not raw_path:
        return Path.home().resolve()

    expanded = Path(os.path.expanduser(raw_path))
    if expanded.is_absolute():
        return expanded.resolve()
    return (STATE.cwd / expanded).resolve()


def resolve_send_path(raw_path: str) -> Path:
    expanded = Path(os.path.expanduser(raw_path))
    if expanded.is_absolute():
        return expanded.resolve()
    return (Path.home() / expanded).resolve()


def parse_single_shell_path(raw_path: str) -> str | None:
    try:
        tokens = shlex.split(raw_path)
    except ValueError:
        return None
    if len(tokens) != 1:
        return None
    return tokens[0]


async def run_shell_command(command: str) -> str:
    proc = await asyncio.create_subprocess_shell(
        command,
        cwd=str(STATE.cwd),
        executable="/bin/bash",
        stdout=asyncio.subprocess.PIPE,
        stderr=asyncio.subprocess.PIPE,
        start_new_session=True,
    )

    timed_out = False
    try:
        stdout, stderr = await asyncio.wait_for(
            proc.communicate(), timeout=COMMAND_TIMEOUT_SECONDS
        )
    except asyncio.TimeoutError:
        timed_out = True
        try:
            os.killpg(proc.pid, signal.SIGTERM)
        except ProcessLookupError:
            pass

        try:
            stdout, stderr = await asyncio.wait_for(proc.communicate(), timeout=5)
        except asyncio.TimeoutError:
            try:
                os.killpg(proc.pid, signal.SIGKILL)
            except ProcessLookupError:
                pass
            stdout, stderr = await proc.communicate()

    stdout_text = stdout.decode(errors="replace")
    stderr_text = stderr.decode(errors="replace")
    parts: list[str] = []

    status = f"exit={proc.returncode}"
    if timed_out:
        status += f" timeout={COMMAND_TIMEOUT_SECONDS}s"
    parts.append(status)

    if stdout_text:
        parts.append(f"stdout:\n{stdout_text.rstrip()}")
    if stderr_text:
        parts.append(f"stderr:\n{stderr_text.rstrip()}")

    return "\n\n".join(parts)


async def handle_start(update: Update, context: ContextTypes.DEFAULT_TYPE) -> None:
    if not is_admin(update):
        return
    await send_text_chunks(update, "Admin shell bot online. Send /help for commands.")


async def handle_help(update: Update, context: ContextTypes.DEFAULT_TYPE) -> None:
    if not is_admin(update):
        return

    await send_text_chunks(
        update,
        "\n".join(
            [
                "Commands:",
                "/start - show status",
                "/help - show this help",
                "/pwd - show current working directory",
                "/cd [path] - change working directory",
                "/cmd <bash> - run a bash command",
                "/send <path> - send file from home-relative, ~, or absolute path",
                "",
                "Plain text messages are also executed as bash commands.",
            ]
        ),
    )


async def handle_pwd(update: Update, context: ContextTypes.DEFAULT_TYPE) -> None:
    if not is_admin(update):
        return
    await send_text_chunks(update, str(STATE.cwd))


async def handle_cd(update: Update, context: ContextTypes.DEFAULT_TYPE) -> None:
    if not is_admin(update):
        return

    raw_path = " ".join(context.args).strip()
    parsed_path = parse_single_shell_path(raw_path) if raw_path else ""
    if parsed_path is None:
        await send_text_chunks(update, "Usage: /cd [single path]")
        return

    target = resolve_cd_path(parsed_path)
    if not target.exists():
        await send_text_chunks(update, f"cd: no such path: {target}")
        return
    if not target.is_dir():
        await send_text_chunks(update, f"cd: not a directory: {target}")
        return

    STATE.cwd = target
    await send_text_chunks(update, str(STATE.cwd))


async def handle_cmd(update: Update, context: ContextTypes.DEFAULT_TYPE) -> None:
    if not is_admin(update):
        return

    text = update.effective_message.text
    parts = text.split(maxsplit=1)
    command = parts[1].strip() if len(parts) == 2 else ""
    if not command:
        await send_text_chunks(update, "Usage: /cmd <bash command>")
        return

    await run_and_reply(update, context, command)


async def handle_plain_text(update: Update, context: ContextTypes.DEFAULT_TYPE) -> None:
    if not is_admin(update):
        return

    message = update.effective_message
    if not message or not message.text:
        return

    await run_and_reply(update, context, message.text)


async def run_and_reply(
    update: Update, context: ContextTypes.DEFAULT_TYPE, command: str
) -> None:
    if update.effective_chat:
        await context.bot.send_chat_action(
            chat_id=update.effective_chat.id, action=ChatAction.TYPING
        )
    result = await run_shell_command(command)
    await send_text_chunks(update, result)


def guess_media_kind(path: Path) -> str:
    mime_type, _ = mimetypes.guess_type(path.name)
    suffix = path.suffix.lower()

    if mime_type:
        if mime_type.startswith("image/"):
            return "photo"
        if mime_type.startswith("video/"):
            return "video"
        if mime_type.startswith("audio/"):
            if suffix in {".oga", ".ogg", ".opus"}:
                return "voice"
            return "audio"

    if suffix in {".jpg", ".jpeg", ".png", ".webp", ".gif", ".bmp", ".tif", ".tiff"}:
        return "photo"
    if suffix in {".mp4", ".mov", ".mkv", ".webm", ".avi", ".m4v"}:
        return "video"
    if suffix in {".mp3", ".m4a", ".flac", ".wav", ".aac"}:
        return "audio"
    if suffix in {".oga", ".ogg", ".opus"}:
        return "voice"
    return "document"


async def handle_send(update: Update, context: ContextTypes.DEFAULT_TYPE) -> None:
    if not is_admin(update):
        return

    raw_path = " ".join(context.args).strip()
    if not raw_path:
        await send_text_chunks(update, "Usage: /send <home-relative|~|absolute path>")
        return

    parsed_path = parse_single_shell_path(raw_path)
    if parsed_path is None:
        await send_text_chunks(update, "Usage: /send <single path>")
        return

    path = resolve_send_path(parsed_path)
    if not path.exists():
        await send_text_chunks(update, f"/send: no such file: {path}")
        return
    if not path.is_file():
        await send_text_chunks(update, f"/send: not a regular file: {path}")
        return

    chat = update.effective_chat
    if not chat:
        return

    media_kind = guess_media_kind(path)
    senders: dict[str, Callable[..., Awaitable[object]]] = {
        "photo": context.bot.send_photo,
        "video": context.bot.send_video,
        "audio": context.bot.send_audio,
        "voice": context.bot.send_voice,
        "document": context.bot.send_document,
    }
    payload_name = {
        "photo": "photo",
        "video": "video",
        "audio": "audio",
        "voice": "voice",
        "document": "document",
    }[media_kind]

    await context.bot.send_chat_action(chat_id=chat.id, action=ChatAction.UPLOAD_DOCUMENT)
    try:
        with path.open("rb") as file_obj:
            await senders[media_kind](
                chat_id=chat.id,
                **{payload_name: file_obj},
                caption=path.name if media_kind != "voice" else None,
            )
    except Exception as exc:
        await send_text_chunks(update, f"/send failed as {media_kind}: {exc}")


def build_application() -> Application:
    if TELEGRAM_IMPORT_ERROR is not None:
        print(
            "Missing dependency: python-telegram-bot>=20\n"
            'Install with: python3 -m pip install "python-telegram-bot>=20"',
            file=sys.stderr,
        )
        raise SystemExit(2)

    application = Application.builder().token(BOT_TOKEN).build()
    application.add_handler(CommandHandler("start", handle_start))
    application.add_handler(CommandHandler("help", handle_help))
    application.add_handler(CommandHandler("pwd", handle_pwd))
    application.add_handler(CommandHandler("cd", handle_cd))
    application.add_handler(CommandHandler("send", handle_send))
    application.add_handler(CommandHandler("cmd", handle_cmd))
    application.add_handler(MessageHandler(filters.TEXT, handle_plain_text))
    return application


def parse_args(argv: list[str]) -> argparse.Namespace:
    parser = argparse.ArgumentParser(
        description="Polling-only Telegram admin shell bot."
    )
    parser.add_argument(
        "--timeout",
        type=int,
        default=COMMAND_TIMEOUT_SECONDS,
        help=f"bash command timeout in seconds (default: {COMMAND_TIMEOUT_SECONDS})",
    )
    return parser.parse_args(argv)


def main(argv: list[str] | None = None) -> int:
    global COMMAND_TIMEOUT_SECONDS

    args = parse_args(sys.argv[1:] if argv is None else argv)
    COMMAND_TIMEOUT_SECONDS = args.timeout

    application = build_application()
    application.run_polling(allowed_updates=Update.ALL_TYPES)
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
