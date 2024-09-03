import os
import ccxt
import telebot
import schedule
import time
import logging
from datetime import datetime, timedelta
import pytz

singapore_tz = pytz.timezone('Asia/Singapore')
exchange = ccxt.binance()
BOT_TOKEN = os.environ['BOT_TOKEN']
CHAT_ID = os.environ['CHAT_ID']
bot = telebot.TeleBot(BOT_TOKEN)
SYMBOLS = os.environ['SYMBOLS'].split(',')

def get_ticker_info(symbol):
    ticker = exchange.fetch_ticker(symbol)
    return {
        'symbol': symbol,
        'last': ticker['last'],
        'change_percent': ticker['percentage'],
        'high': ticker['high'],
        'low': ticker['low'],
        'volume': ticker['baseVolume'],
        'quote_volume': ticker['quoteVolume'],
        'bid': ticker['bid'],
        'ask': ticker['ask']
    }
def format_change(change_percent):
    if change_percent > 0:
        return f"🔼 +{change_percent:.2f}%"
    elif change_percent < 0:
        return f"🔽 {change_percent:.2f}%"
    else:
        return f"◀▶ {change_percent:.2f}%"
def send_price_update():
    now = datetime.now(singapore_tz)
    message = f"市场更新 - {now.strftime('%Y-%m-%d %H:%M:%S')} (SGT)\n\n"
    
    for symbol in SYMBOLS:
        info = get_ticker_info(symbol)
        change_str = format_change(info['change_percent'])
        
        message += f"*{info['symbol']}*\n"
        message += f"价格: ${info['last']:.7f}\n"
        message += f"24h 涨跌: {change_str}\n"
        message += f"24h 高/低: ${info['high']:.7f} / ${info['low']:.7f}\n"
        message += f"24h 成交量: {info['volume']:.2f}\n"
        message += f"24h 成交额: ${info['quote_volume']:.2f}\n"
        message += f"买一/卖一: ${info['bid']:.7f} / ${info['ask']:.7f}\n\n"
    
    bot.send_message(CHAT_ID, message, parse_mode='Markdown')

def run():
    logging.basicConfig(level=logging.INFO, format='%(asctime)s - %(name)s - %(levelname)s - %(message)s')
    logger = logging.getLogger('BinanceUpdater')
    
    while True:
        try:
            # 立即执行一次价格更新
            logger.info("Sending initial price update...")
            send_price_update()

            # 设置定时任务，每小时整点执行
            for hour in range(24):
                schedule.every().day.at(f"{hour:02d}:00").do(send_price_update)

            logger.info("Scheduled tasks set. Waiting for next hour...")

            # 等待下一个整点
            now = datetime.now(singapore_tz)
            next_hour = (now + timedelta(hours=1)).replace(minute=0, second=0, microsecond=0)
            time.sleep((next_hour - now).total_seconds())

            logger.info("Starting main loop...")

            while True:
                schedule.run_pending()
                time.sleep(30)  # 每30秒检查一次，可以根据需要调整
        except Exception as e:
            logger.error(f"An error occurred in BinanceUpdater: {str(e)}")
            logger.info("Attempting to restart BinanceUpdater in 60 seconds...")
            time.sleep(60)  # 等待60秒后重试

if __name__ == '__main__':
    run()
