import asyncio
import logging
from aiogram import Bot, Dispatcher, types, F
from aiogram.filters import Command
from aiogram.types import InlineKeyboardMarkup, InlineKeyboardButton
from aiogram.enums import ParseMode

from config import BOT_TOKEN, ADMIN_ID, PRICE_RUB, PRICE_LABEL, DOWNLOAD_MAC_URL, DOWNLOAD_WIN_URL
import db

logging.basicConfig(level=logging.INFO)
bot = Bot(token=BOT_TOKEN, parse_mode=ParseMode.HTML)
dp = Dispatcher()

# ─── Keyboards ───

def main_kb():
    return InlineKeyboardMarkup(inline_keyboard=[
        [InlineKeyboardButton(text=f"💳 Купить Pro ({PRICE_LABEL})", callback_data="buy")],
        [InlineKeyboardButton(text="📥 Скачать бесплатную версию", callback_data="download_free")],
        [InlineKeyboardButton(text="🔑 Мой ключ", callback_data="my_key")],
        [InlineKeyboardButton(text="💬 Обратная связь", callback_data="feedback")],
    ])

def buy_kb():
    return InlineKeyboardMarkup(inline_keyboard=[
        [InlineKeyboardButton(text="💳 Перевод на карту / СБП", callback_data="pay_card")],
        [InlineKeyboardButton(text="◀️ Назад", callback_data="back_main")],
    ])

def download_kb():
    return InlineKeyboardMarkup(inline_keyboard=[
        [InlineKeyboardButton(text="🍎 macOS (.dmg)", url=DOWNLOAD_MAC_URL)],
        [InlineKeyboardButton(text="🪟 Windows (.exe)", url=DOWNLOAD_WIN_URL)],
        [InlineKeyboardButton(text="◀️ Назад", callback_data="back_main")],
    ])

def admin_confirm_kb(user_id):
    return InlineKeyboardMarkup(inline_keyboard=[
        [InlineKeyboardButton(text="✅ Подтвердить оплату", callback_data=f"confirm_{user_id}")],
        [InlineKeyboardButton(text="❌ Отклонить", callback_data=f"reject_{user_id}")],
    ])

# ─── /start ───

@dp.message(Command("start"))
async def cmd_start(msg: types.Message):
    await msg.answer(
        "👋 <b>Bzz</b> — автопереключатель раскладки\n\n"
        "Печатаешь <code>ghbdtn</code> → получаешь <b>привет</b>.\n"
        "Работает на macOS и Windows.\n\n"
        "Бесплатная версия — автопереключение RU↔EN.\n"
        f"Pro ({PRICE_LABEL} навсегда, v0.4) — свой словарь, доп. раскладки (UK/KZ/BY/DE/FR), графический менеджер исключений.\n\n"
        "⚠️ Pro версия в разработке (v0.4, релиз Q3 2026). "
        "Сейчас бесплатное ядро покрывает 90% потребностей. "
        "Поддержать разработку Pro можно на Boosty.",
        reply_markup=main_kb()
    )

# ─── Buy flow ───

@dp.callback_query(F.data == "buy")
async def cb_buy(cb: types.CallbackQuery):
    existing = db.has_license(cb.from_user.id)
    if existing:
        await cb.message.edit_text(
            f"✅ У тебя уже есть лицензия!\n\n🔑 <code>{existing}</code>",
            reply_markup=InlineKeyboardMarkup(inline_keyboard=[
                [InlineKeyboardButton(text="◀️ Назад", callback_data="back_main")]
            ])
        )
        return
    await cb.message.edit_text(
        f"💳 <b>Bzz Pro — {PRICE_LABEL} (навсегда)</b>\n\n"
        "Выбери способ оплаты:",
        reply_markup=buy_kb()
    )

@dp.callback_query(F.data == "pay_card")
async def cb_pay_card(cb: types.CallbackQuery):
    db.save_pending(cb.from_user.id, cb.from_user.username, cb.from_user.full_name)

    await cb.message.edit_text(
        f"💳 <b>Оплата {PRICE_LABEL}</b>\n\n"
        "Переведи на карту или по СБП:\n\n"
        "📱 <b>СБП:</b> +7 (XXX) XXX-XX-XX (Тинькофф)\n"
        "💳 <b>Карта:</b> <code>XXXX XXXX XXXX XXXX</code>\n\n"
        "После оплаты нажми кнопку ниже 👇",
        reply_markup=InlineKeyboardMarkup(inline_keyboard=[
            [InlineKeyboardButton(text="✅ Я оплатил", callback_data="i_paid")],
            [InlineKeyboardButton(text="◀️ Отмена", callback_data="back_main")],
        ])
    )

@dp.callback_query(F.data == "i_paid")
async def cb_i_paid(cb: types.CallbackQuery):
    user = cb.from_user
    await cb.message.edit_text(
        "⏳ Отлично! Ожидай подтверждения — обычно занимает несколько минут.\n\n"
        "Как только оплата подтвердится, я пришлю лицензионный ключ."
    )

    # Notify admin
    await bot.send_message(
        ADMIN_ID,
        f"💰 <b>Новая оплата!</b>\n\n"
        f"👤 {user.full_name} (@{user.username})\n"
        f"🆔 <code>{user.id}</code>\n"
        f"💵 {PRICE_LABEL}\n\n"
        "Подтвердить?",
        reply_markup=admin_confirm_kb(user.id)
    )

# ─── Admin confirms ───

@dp.callback_query(F.data.startswith("confirm_"))
async def cb_confirm(cb: types.CallbackQuery):
    if cb.from_user.id != ADMIN_ID:
        return

    user_id = int(cb.data.split("_")[1])
    key = db.generate_key(user_id)

    # Get user info from pending
    conn = db.get_conn()
    row = conn.execute("SELECT * FROM pending_payments WHERE user_id=? AND status='pending'", (user_id,)).fetchone()
    conn.close()

    username = row["username"] if row else "unknown"
    full_name = row["full_name"] if row else "unknown"

    db.confirm_pending(user_id)
    db.save_license(user_id, username, full_name, key, "card")

    # Send key to user
    await bot.send_message(
        user_id,
        f"🎉 <b>Оплата подтверждена!</b>\n\n"
        f"Твой лицензионный ключ:\n🔑 <code>{key}</code>\n\n"
        "Скачай приложение и введи ключ при первом запуске.",
        reply_markup=download_kb()
    )

    await cb.message.edit_text(
        f"✅ Подтверждено для {full_name} (@{username})\n🔑 <code>{key}</code>"
    )

    stats = db.get_stats()
    await bot.send_message(ADMIN_ID, f"📊 Всего лицензий: {stats['licenses']}")

@dp.callback_query(F.data.startswith("reject_"))
async def cb_reject(cb: types.CallbackQuery):
    if cb.from_user.id != ADMIN_ID:
        return

    user_id = int(cb.data.split("_")[1])
    db.confirm_pending(user_id)  # just clear pending

    await bot.send_message(
        user_id,
        "❌ К сожалению, оплата не подтверждена. Если ты считаешь что это ошибка — напиши в обратную связь."
    )
    await cb.message.edit_text("❌ Отклонено")

# ─── Download ───

@dp.callback_query(F.data == "download_free")
async def cb_download(cb: types.CallbackQuery):
    await cb.message.edit_text(
        "📥 <b>Скачать Bzz</b>\n\n"
        "Бесплатная версия включает автопереключение RU↔EN.\n"
        "Pro-фичи активируются лицензионным ключом.",
        reply_markup=download_kb()
    )

# ─── My key ───

@dp.callback_query(F.data == "my_key")
async def cb_my_key(cb: types.CallbackQuery):
    key = db.has_license(cb.from_user.id)
    if key:
        await cb.message.edit_text(
            f"🔑 Твой лицензионный ключ:\n\n<code>{key}</code>",
            reply_markup=InlineKeyboardMarkup(inline_keyboard=[
                [InlineKeyboardButton(text="◀️ Назад", callback_data="back_main")]
            ])
        )
    else:
        await cb.message.edit_text(
            "У тебя пока нет лицензии.",
            reply_markup=InlineKeyboardMarkup(inline_keyboard=[
                [InlineKeyboardButton(text=f"💳 Купить Pro ({PRICE_LABEL})", callback_data="buy")],
                [InlineKeyboardButton(text="◀️ Назад", callback_data="back_main")],
            ])
        )

# ─── Feedback ───

waiting_feedback = set()

@dp.callback_query(F.data == "feedback")
async def cb_feedback(cb: types.CallbackQuery):
    waiting_feedback.add(cb.from_user.id)
    await cb.message.edit_text(
        "💬 Напиши свой отзыв, баг или предложение — я передам разработчику.",
        reply_markup=InlineKeyboardMarkup(inline_keyboard=[
            [InlineKeyboardButton(text="◀️ Отмена", callback_data="back_main")]
        ])
    )

@dp.message(F.text & ~F.text.startswith("/"))
async def handle_text(msg: types.Message):
    if msg.from_user.id in waiting_feedback:
        waiting_feedback.discard(msg.from_user.id)
        db.save_feedback(msg.from_user.id, msg.from_user.username, msg.text)
        await msg.answer("✅ Спасибо! Обратная связь отправлена разработчику.")

        await bot.send_message(
            ADMIN_ID,
            f"💬 <b>Обратная связь</b>\n\n"
            f"👤 {msg.from_user.full_name} (@{msg.from_user.username})\n\n"
            f"{msg.text}"
        )

# ─── Back ───

@dp.callback_query(F.data == "back_main")
async def cb_back(cb: types.CallbackQuery):
    waiting_feedback.discard(cb.from_user.id)
    await cb.message.edit_text(
        "👋 <b>Bzz</b> — автопереключатель раскладки\n\n"
        f"Бесплатная версия — автопереключение RU↔EN.\n"
        f"Pro ({PRICE_LABEL} навсегда) — хоткей исправления, whitelist, доп. языки.",
        reply_markup=main_kb()
    )

# ─── Admin commands ───

@dp.message(Command("stats"))
async def cmd_stats(msg: types.Message):
    if msg.from_user.id != ADMIN_ID:
        return
    stats = db.get_stats()
    await msg.answer(
        f"📊 <b>Статистика Bzz</b>\n\n"
        f"🔑 Лицензий: {stats['licenses']}\n"
        f"⏳ Ожидает подтверждения: {stats['pending']}\n"
        f"💬 Отзывов: {stats['feedback']}\n"
        f"💰 Доход: ~{stats['licenses'] * PRICE_RUB} ₽"
    )

@dp.message(Command("pending"))
async def cmd_pending(msg: types.Message):
    if msg.from_user.id != ADMIN_ID:
        return
    rows = db.get_pending()
    if not rows:
        await msg.answer("Нет ожидающих платежей.")
        return
    for row in rows:
        await msg.answer(
            f"⏳ {row['full_name']} (@{row['username']})\n🆔 <code>{row['user_id']}</code>",
            reply_markup=admin_confirm_kb(row['user_id'])
        )

async def main():
    db.init_db()
    logging.info("Bzz Bot started")
    await dp.start_polling(bot)

if __name__ == "__main__":
    asyncio.run(main())
