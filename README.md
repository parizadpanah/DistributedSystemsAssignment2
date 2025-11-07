
# KVStore — تمرین دوم سیستم‌های توزیع‌شده 

این پروژه طراحی و پیاده‌سازی یک پایگاه داده **Key–Value Store** است که با زبان **Golang** نوشته شده. داده‌ها در فایل‌های `JSONL` ذخیره می‌شوند که بتوان بعد از ری‌استارت کردن داده ها باقی بمانند.
این سرویس **HTTP** با دو Endpoint اصلی و یک Endpoint اختیاری ارائه شده است.
1. `PUT /objects`
2. `GET /objects{key}`
3. `GET /objects`

 1: در این بخش ذخیره سازی داده ها صورت میگیرد با شروطی که در توضیحات پروژه آماده است باید این ذخیره سازی انجام شود. تابع **put** در main وظیفه انجام اینکار را دارد.<br>
 2: در این بخش بازیابی داده با داشتن key صورت میگیرد، به دنبال این کلید در کالکشن ها گشته تا در صورت وجود و عدم وجود پیغام با فرمت مناسب چاپ کند؛ تابع **get** در main وظیفه انجام اینکار را بر عهده دارد.<br>
 3: در این بخش میتوان به مشاهده اطلاعات ذخیره شده داده در مجموعه های مختلف(collection) بپردازد. در این بخش لیستی از همه اشیا با فیلتر**prefix** و با **صفحه‌بندی** انجام گیرد.تابع **list** در main وظیفه انجام اینکار را دارد.<br>

- پیاده سازی **Collection**

 این بخش برای گروهبندی داده‌ها `?collection=` در کد نوشته شده است که میتوان داده ها را در دسته بندی و مجموعه های مختلف ذخیره سازی کرد اگر از collection در کد نویسی استفاده نوشد به صورت defult ست میشود. که توابع **openStore** و **openCollection** برای ساخت یک فروشگاه و ساخت یا ابز کردن یک کالکشن است.

 
- بهینه‌سازی‌های **Performance**

رعایت پرفورمنس بالا در ذخیره سازی و بازیابی در چند جای کد تلاش شده که رعایت شود از موارد موجود میتوان به:ایندکس در حافظه، نوشتن append-only، **Streaming JSON** در لیست، و **Compaction خودکار** و... اشاره کرد.

> **ساختار داده روی دیسک:**
>  برای هر کالکشن یک فایل در مسیر `data/<collection>.jsonl` وجود دارد. اگر کالکشن ندهید، مقدار پیش‌فرض `default` است.

---

## 1) پیش‌نیازها
- **Go 1.22+**
- **Docker**

---

## 2) اجرای محلی (بدون Docker)

### 2.1. اجرا
```bash
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

- **لیست همهٔ اشیاء** :
```bash
# همهٔ کالکشن‌ها با نمایش نام کالکشن
curl "http://localhost:8080/objects?limit=200&includeCollection=true"

# فقط کالکشن users
curl "http://localhost:8080/objects?collection=users&limit=50&offset=0"

# فیلتر پیشوندی (مثلاً کلیدهایی که با user: شروع می‌شوند)
curl "http://localhost:8080/objects?prefix=user:&limit=200&includeCollection=true"

# گرفتن یک محصول خاص
curl.exe "http://localhost:8080/objects/p:1001?collection=products"
```

---

## 3) اجرای Docker

### 3.1. ساخت ایمیج
```bash
docker build -t kvstore:opt .
```

### 3.2. اجرا با پایداری داده
```bash
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
```bash
curl.exe -X PUT "http://localhost:8080/objects?collection=products" -H "Content-Type: application/json"
--data "{\"key\":\"p:1001\",\"value\":{\"title\":\"Keyboard\",\"price\":39.9,\"stock\":15,\"tags\":[\"peripheral\",\"wired\"]}}"
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
- **Query**: `collection=<name>` اختیاری (اگر در کالکشنی ذخیره کرده باشیم همان را می دهیم)
- **Response**:
  - موجود: `200 OK` + مقدار JSON خام
  - ناموجود: `404 Not Found`

### GET `/objects`
- **Query**:
  - `collection=<name>` (اختیاری؛ اگر ندهیم همهٔ کالکشن‌ها لیست می‌شوند)
  - `limit` (۱..۱۰۰۰۰، پیش‌فرض ۱۰۰) و (`offset` ≥۰)
  - `prefix` برای فیلتر پیشوندی روی کلید
  - `includeCollection=true` برای نمایش نام کالکشن کنار هر آیتم
- **Response**: آرایهٔ JSON به‌صورت **استریم** (حافظهٔ ثابت در مقیاس بزرگ)

---



## 7) اندازهٔ ایمیج و بهینه‌سازی
-**Dockerfile چند مرحله ای**مرحله build روی `golang:1.22-alpine`و مرحله نهایی روی `scratch` برای کمینه کردن سایز انجام میشود.




باینری استاتیک و کم‌حجم: با CGO_ENABLED=0, GOOS=linux, GOARCH=amd64 و -trimpath -ldflags "-s -w" کامپایل می‌شود.

اجرای امن (nonroot) و بدون شِل: در stage نهایی USER 65532:65532 استفاده شده و چون base scratch است، شِل یا ابزار اضافی وجود ندارد (surface کوچک‌تر).

پیکربندی سادهٔ اجرا: متغیرها APP_ADDR=:8080 و DATA_DIR=/app/data؛ پورت سرویس EXPOSE 8080.

ماندگاری داده: توصیه به mount کردن مسیر داده در زمان اجرا: -v %cd%/data:/app/data (یا معادل لینوکسی).

برای نمایش سایز ایمیج:
```bash
docker images kvstore:opt
```
