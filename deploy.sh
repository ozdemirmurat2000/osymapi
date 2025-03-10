#!/bin/bash

# Hata durumunda script'i durdur
set -e

# Değişkenler
APP_NAME="osymapp"
APP_DIR="/var/www/$APP_NAME"
REPO_URL="https://github.com/your-username/$APP_NAME.git"
LOG_FILE="/var/log/$APP_NAME/deploy.log"

# Log dosyası için dizin oluştur
mkdir -p "$(dirname $LOG_FILE)"

# Loglama fonksiyonu
log() {
    echo "[$(date '+%Y-%m-%d %H:%M:%S')] $1" | tee -a "$LOG_FILE"
}

# Uygulama dizinini oluştur
if [ ! -d "$APP_DIR" ]; then
    log "Creating application directory..."
    mkdir -p "$APP_DIR"
    cd "$APP_DIR"
    git clone "$REPO_URL" .
else
    log "Updating existing repository..."
    cd "$APP_DIR"
    git fetch origin
    git reset --hard origin/main
fi

# .env dosyasını koru
if [ ! -f ".env" ]; then
    log "Creating .env file..."
    cp .env.example .env
    # TODO: .env dosyasını düzenleyin
fi

# Go bağımlılıklarını yükle
log "Installing dependencies..."
go mod download

# Uygulamayı derle
log "Building application..."
go build -o $APP_NAME

# Systemd service dosyasını oluştur
if [ ! -f "/etc/systemd/system/$APP_NAME.service" ]; then
    log "Creating systemd service..."
    cat > "/etc/systemd/system/$APP_NAME.service" <<EOF
[Unit]
Description=OSYM App API Server
After=network.target

[Service]
Type=simple
User=www-data
WorkingDirectory=$APP_DIR
ExecStart=$APP_DIR/$APP_NAME
Restart=always
Environment=ENV=production

[Install]
WantedBy=multi-user.target
EOF

    # Servis kullanıcısına izinleri ver
    chown -R www-data:www-data "$APP_DIR"
    chmod +x "$APP_DIR/$APP_NAME"
fi

# Systemd'yi yeniden yükle ve servisi başlat
log "Restarting service..."
systemctl daemon-reload
systemctl restart $APP_NAME
systemctl enable $APP_NAME

# Servis durumunu kontrol et
log "Service status:"
systemctl status $APP_NAME --no-pager

log "Deployment completed successfully!" 