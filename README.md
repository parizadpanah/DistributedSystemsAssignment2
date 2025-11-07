
# KVStore — تمرین دوم سیستم‌های توزیع‌شده 

این پروژه طراحی و پیاده‌سازی یک پایگاه داده **Key–Value Store** است که با زبان **Golang** نوشته شده. داده‌ها در فایل‌های `JSONL` ذخیره می‌شوند کهبتوان بعد از ری‌استارت کردن داده ها باقی بمانند.  
این سرویس **HTTP** با دو Endpoint اصلی و یک Endpoint اختیاری ارائه شده است.
- `PUT /objects`
- `GET /objects{key}`
- `GET /objects`
1: در این بخش ذخیره سازی داده ها صورت میگیرد با شروطی که در توضیحات پروژه آماده است باید این ذخیره سازی انجام شود. تابع **put** در main وظیفه انجام اینکار را دارد.
2: در این بخش بازیابی داده با داشتن key صورت میگیرد، به دنبال این کلید در کالکشن ها گشته تا در صورت وجود و عدم وجود پیغام با فرمت مناسب چاپ کند؛ تابع **get** در main وظیفه انجام اینکار را بر عهده دارد.
3:در این بخش میتوان به مشاهده اطلاعات ذخیره شده داده در مجموعه های مختلف(collection) بپردازد. در این بخش لیستی از همه اشیا با فیلتر**`prefix`** و با **صفحه‌بندی** انجام گیرد.تابع**list** در main وظیفه انجام اینکار را دارد.

- `GET /objects` برای لیست‌کردن همهٔ اشیاء (با **صفحه‌بندی** و فیلتر **`prefix`**)
- **Collection** برای گروهبندی داده‌ها (مانند جدول/namespace) با پارامتر `?collection=`
- بهینه‌سازی‌های **Performance**: ایندکس در حافظه، نوشتن append-only، **Streaming JSON** در لیست، و **Compaction خودکار**

> **ساختار داده روی دیسک:** برای هر کالکشن یک فایل `data/<collection>.jsonl` وجود دارد. اگر کالکشن ندهید، مقدار پیش‌فرض `default` است.

---

## 1) پیش‌نیازها
- **Go 1.22+**
- (اختیاری برای استقرار) **Docker**

---

## 2) اجرای محلی (بدون Docker)

### 2.1. اجرا
```bash
# Linux/Mac
export APP_ADDR=":8080"
export DATA_DIR="./data"
go run ./main.go

# Windows - PowerShell
$env:APP_ADDR=":8080"; $env:DATA_DIR="$PWD\data"; go run .\main.go

# Windows - CMD
set APP_ADDR=:8080 & set DATA_DIR=%cd%\data & go run .\main.go
```
پس از اجرا باید ببینید:
```
listening on :8080 (data: ./data)
```

### 2.2. تست سریع با curl
> برای دیدن **Status-Line** مثل `HTTP/1.1 200 OK` از سویچ `-i` استفاده کنید.

- **ذخیره** (کالکشن پیش‌فرض):
```bash
curl -i -X PUT "http://localhost:8080/objects" \
  -H "Content-Type: application/json" \
  --data '{"key":"user:1234","value":{"name":"Amin Alavli","age":23,"email":"a.alavi@fum.ir"}}'
```

- **گرفتن با کلید** (وجود دارد → 200 OK):
```bash
curl -i "http://localhost:8080/objects/user:1234"
```
خروجی نمونه:
```
HTTP/1.1 200 OK
Content-Type: application/json
...
{"name":"Amin Alavli","age":23,"email":"a.alavi@fum.ir"}
```

- **گرفتن کلید ناموجود** (→ 404 Not Found):
```bash
curl -i "http://localhost:8080/objects/user:9999"
```

- **لیست همهٔ اشیاء** (امتیازی):
```bash
# همهٔ کالکشن‌ها با نمایش نام کالکشن
curl "http://localhost:8080/objects?limit=200&includeCollection=true"

# فقط کالکشن users
curl "http://localhost:8080/objects?collection=users&limit=50&offset=0"

# فیلتر پیشوندی (مثلاً کلیدهایی که با user: شروع می‌شوند)
curl "http://localhost:8080/objects?prefix=user:&limit=200&includeCollection=true"
```

---

## 3) اجرای Docker (ایمیج کم‌حجم)

### 3.1. ساخت ایمیج
```bash
docker build -t kvstore:opt .
```

### 3.2. اجرا با پایداری داده
```bash
# Linux/Mac
docker run --rm -p 8080:8080 -e DATA_DIR=/data -v "$PWD/data:/data" kvstore:opt

# Windows - PowerShell
docker run --rm -p 8080:8080 -e DATA_DIR=/data -v "${PWD}\data:/data" kvstore:opt

# Windows - CMD
docker run --rm -p 8080:8080 -e DATA_DIR=/data -v "%cd%\data:/data" kvstore:opt
```
> با `-v` داده‌ها روی دیسک میزبان ذخیره می‌شوند و بعد از توقف کانتینر باقی می‌مانند.

### 3.3. تست داخل Docker
مثل اجرای محلی:
```bash
curl -i "http://localhost:8080/objects?limit=1"
curl -i -X PUT "http://localhost:8080/objects" -H "Content-Type: application/json" \
  --data '{"key":"user:1234","value":{"name":"Amin Alavli","age":23,"email":"a.alavi@fum.ir"}}'
```

---

## 4) API (خلاصه)

### PUT `/objects`
- **Body** (`application/json`):
```json
{"key":"<string>","value": <valid-json>}
```
- **Query**: `collection=<name>` اختیاری (پیش‌فرض: `default`)
- **Response**: `HTTP/1.1 200 OK` (بدنهٔ پیش‌فرض ساده: `{"status":"ok"}`)

### GET `/objects/{key}`
- **Query**: `collection=<name>` اختیاری (اگر در کالکشنی ذخیره کرده‌اید، همان را بدهید)
- **Response**:
  - موجود: `200 OK` + مقدار JSON خام
  - ناموجود: `404 Not Found`

### GET `/objects`
- **Query**:
  - `collection=<name>` (اختیاری؛ اگر ندهید همهٔ کالکشن‌ها لیست می‌شوند)
  - `limit` (۱..۱۰۰۰۰، پیش‌فرض ۱۰۰) و `offset` (≥۰)
  - `prefix` برای فیلتر پیشوندی روی کلید
  - `includeCollection=true` برای نمایش نام کالکشن کنار هر آیتم
- **Response**: آرایهٔ JSON به‌صورت **استریم** (حافظهٔ ثابت در مقیاس بزرگ)

---

## 5) Optional Features (امتیازی) — چه کرده‌ایم؟
- **Performance (small/large scale):**
  - In-memory index برای `GET`‌ سریع
  - Append-only JSONL برای write سریع
  - **Streaming JSON** در `GET /objects` (بدون ساخت آرایهٔ بزرگ در حافظه)
  - **Auto-Compaction**: وقتی فایل «باد» می‌کند، نسخهٔ فشردهٔ فقط-آخرین رکوردها ساخته و جایگزین می‌شود
- **Collection**: گروهبندی منطقی با `?collection=...` (هر کالکشن یک فایل جدا)
- **`GET /objects`**: پیاده‌سازی کامل با صفحه‌بندی، فیلتر `prefix` و سویچ `includeCollection`

---

## 6) نکات مهم و تفاوت‌ها
- اگر با `?collection=users` ذخیره کنید، در `GET /objects/{key}` هم باید همان `collection` را بدهید؛ در غیر این صورت 404 می‌گیرید.
- برای لیست‌کردن **همهٔ کالکشن‌ها** پارامتر `collection` را حذف کنید (و برای شفافیت، `includeCollection=true` بگذارید).
- **Status-Line** (مثل `HTTP/1.1 200 OK`) با `curl -i` نشان داده می‌شود؛ بدنهٔ موفقیت قابل تغییر است (می‌توانید فقط 200 بدون بدنه بدهید).

---

## 7) اسکریپت‌های کمکی (اختیاری)
- **Seed محصولات** در کالکشن `products`: فایل `seed_products.ps1` چند نمونه محصول `p:1001..p:1004` اضافه می‌کند و لیست می‌گیرد:
```powershell
powershell -ExecutionPolicy Bypass -File .\seed_products.ps1 -Base "http://localhost:8080"
```

---

## 8) اندازهٔ ایمیج و بهینه‌سازی
- **Dockerfile چندمرحله‌ای**: مرحلهٔ build روی `golang:1.22-alpine` و مرحلهٔ اجرا روی `distroless:nonroot`
- باینری **استاتیک** با `CGO_ENABLED=0` و `-ldflags "-s -w"` → ایمیج نهایی کوچک و امن
- اجرای **nonroot** و بدون شل (surface attack کم‌تر)

برای نمایش سایز ایمیج:
```bash
docker images kvstore-file:opt
```

---

## 9) چک‌لیست تحویل (مطابق جزوه)
- [x] سورس Go تمیز و مستندسازی‌شده
- [x] فایل **README** با روش اجرا و مثال‌های درخواست/پاسخ
- [x] **Dockerized** با ایمیج کم‌حجم
- [x] **Persistence** (داده‌ها بعد از ری‌استارت باقی می‌مانند)
- [x] سه مورد **اختیاری** (Performance، Collection، `GET /objects`)

---

## 10) خطایابی سریع
- `415 Unsupported Media Type` → هدر `Content-Type: application/json` را گذاشته‌اید؟
- `404 Not Found` در GET-by-key → همان `collection` و همان `key` که ذخیره کردید را می‌خوانید؟
- پورت اشغال است → `APP_ADDR=:9090` و `-p 9090:9090` را استفاده کنید.
- مشاهدهٔ فایل داده:
```powershell
Get-Content .\data\default.jsonl -Tail 10
Get-Content .\data\users.jsonl -Tail 10
Get-Content .\data\products.jsonl -Tail 10
```

---

## 11) لایسنس
این تمرین صرفاً برای اهداف آموزشی/دانشجویی است.
