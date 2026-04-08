import os

BOT_TOKEN = os.getenv("RUSWITCH_BOT_TOKEN", "")
ADMIN_ID = int(os.getenv("RUSWITCH_ADMIN_ID", "7578179116"))  # Roman
PRICE_RUB = 490
PRICE_LABEL = "490 ₽"
DOWNLOAD_MAC_URL = "https://github.com/romankovalev/ruswitch/releases/latest/download/RuSwitch.dmg"
DOWNLOAD_WIN_URL = "https://github.com/romankovalev/ruswitch/releases/latest/download/RuSwitch.exe"
DB_PATH = os.getenv("RUSWITCH_DB_PATH", "ruswitch.db")
