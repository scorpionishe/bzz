import os

BOT_TOKEN = os.getenv("BZZ_BOT_TOKEN", os.getenv("RUSWITCH_BOT_TOKEN", ""))
ADMIN_ID = int(os.getenv("BZZ_ADMIN_ID", os.getenv("RUSWITCH_ADMIN_ID", "7578179116")))  # Roman
PRICE_RUB = 490
PRICE_LABEL = "490 ₽"
DOWNLOAD_MAC_URL = "https://github.com/zlopixatel/bzz/releases/latest/download/Bzz.dmg"
DOWNLOAD_WIN_URL = "https://github.com/zlopixatel/bzz/releases/latest/download/Bzz.exe"
DB_PATH = os.getenv("BZZ_DB_PATH", os.getenv("RUSWITCH_DB_PATH", "bzz.db"))
