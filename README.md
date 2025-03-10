# OSYM Soru Bankası API

Bu proje, OSYM soru bankası için bir REST API sunucusudur.

## Özellikler

- JWT tabanlı kimlik doğrulama
- Rol tabanlı yetkilendirme (Admin, User, Guest)
- Soru yönetimi
- Kategori yönetimi
- Yayıncı yönetimi
- Resim yükleme desteği

## Gereksinimler

- Go 1.19 veya üzeri
- PostgreSQL 13 veya üzeri
- Git

## Kurulum

1. Repoyu klonlayın:
```bash
git clone https://github.com/your-username/osymapp.git
cd osymapp
```

2. Bağımlılıkları yükleyin:
```bash
go mod download
```

3. `.env` dosyasını oluşturun:
```bash
cp .env.example .env
```

4. `.env` dosyasını düzenleyin:
```env
DB_USER=your_db_user
DB_PASSWORD=your_db_password
DB_HOST=localhost
DB_NAME=osymapp
DB_PORT=5432
DB_SSLMODE=disable
JWT_SECRET=your-256-bit-secret-key
```

5. Uygulamayı başlatın:
```bash
go run main.go
```

## API Endpoint'leri

### Kimlik Doğrulama
- `POST /login` - Giriş yap
- `POST /register` - Kayıt ol
- `POST /logout` - Çıkış yap
- `POST /guest-login` - Misafir girişi

### Profil
- `GET /profile` - Profil bilgilerini getir
- `PUT /profile` - Profil bilgilerini güncelle
- `PUT /profile/password` - Şifre değiştir

### Sorular
- `GET /questions` - Soruları listele
- `POST /questions` - Yeni soru ekle
- `PUT /questions/{id}` - Soru güncelle
- `DELETE /questions/{id}` - Soru sil

### Admin İşlemleri
- Kullanıcı yönetimi
- Kategori yönetimi
- Yayıncı yönetimi

## Deployment

Deployment için `deploy.sh` script'ini kullanın:
```bash
./deploy.sh
```

## Lisans

Bu proje özel kullanım içindir ve tüm hakları saklıdır. #   o s y m a p i  
 