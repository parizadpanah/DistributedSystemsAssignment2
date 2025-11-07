
# KV Store (File-based, JSONL) — بدون دیتابیس خارجی

این نسخه **کاملًا فایل‌محور** است (بدون SQL/NoSQL) و سه قابلیت اختیاری را هم پیاده‌سازی می‌کند:

1) **Performance در اسکیل کوچک و بزرگ**: ایندکس در حافظه، نوشتن Append-only روی JSONL،
   **استریم JSON** در لیست، و **Compaction خودکار** وقتی فایل باد می‌کند.
2) **Collection** برای گروهبندی داده‌ها (پارامتر `?collection=`) — هر کالکشن یک فایل جدا در `data/`.
3) **`GET /objects`** برای مشاهدهٔ همهٔ کلیدها/داده‌ها با **صفحه‌بندی** و فیلتر `prefix`.

> مسیر داده‌ها: `DATA_DIR` (پیش‌فرض: `./data`). برای هر collection یک فایل `data/<collection>.jsonl` ساخته می‌شود.

## اجرا (لوکال)
نیازمندی: Go 1.22+

```bash
go run ./main.go
# یا
go build -o kvstore . && ./kvstore
```

متغیرهای محیطی:
- `APP_ADDR` (پیش‌فرض `:8080`)
- `DATA_DIR` (پیش‌فرض `./data`)

## اجرا با Docker
```bash
docker build -t kvstore-file:opt .
docker run --rm -p 8080:8080 -e APP_ADDR=:8080 -e DATA_DIR=/data -v "$PWD/data:/data" kvstore-file:opt
```

## API

### ذخیره/به‌روزرسانی
```
PUT /objects?collection=users
Content-Type: application/json

{"key":"u:1","value":{"name":"Sara","age":21}}
```
- مقدار `value` باید JSON معتبر باشد.
- اگر `collection` ندهید، پیش‌فرض `default` است.

### دریافت با کلید
```
GET /objects/u:1?collection=users
```
- موجود → `200` + بدنهٔ JSON ذخیره‌شده (Raw)
- ناموجود → `404`

### لیست همهٔ اشیاء (امتیازی)
```
GET /objects?collection=users&prefix=u:&limit=50&offset=0
```
پارا‌مترها:
- `collection` (اختیاری) — اگر ندهید، **همهٔ کالکشن‌ها** لیست می‌شوند (فیلد `collection` را می‌توانید با `includeCollection=true` در خروجی ببینید).
- `prefix` (اختیاری) — فیلتر پیشوندی روی کلیدها.
- `limit` (۱..۱۰۰۰۰، پیش‌فرض ۱۰۰) و `offset` (≥۰).

**استریم پاسخ**: خروجی به‌صورت جریان JSON تولید می‌شود تا مصرف حافظهٔ سرور ثابت بماند.

## Performance Notes
- **In-memory index**: آخرین مقدار هر کلید در حافظه نگه‌داری می‌شود تا `GET /objects/{key}` سریع باشد.
- **Append-only JSONL**: فقط به انتهای فایل اضافه می‌شود → write سریع.
- **Auto-Compaction**: اگر نسبت خطوط فایل به تعداد کلیدها زیاد شود (بادکردن)، فایل دوباره‌نویسی می‌شود تا فقط آخرین نسخه‌ها بماند.
- **Streaming list**: `GET /objects` بدون ساخت آرایهٔ بزرگ در حافظه پاسخ را استریم می‌کند.
- **Volume**: با مَونت کردن `DATA_DIR`، داده‌ها پس از ری‌استارت باقی می‌مانند.

## تست سریع
```bash
# 1) ذخیره
curl -X PUT 'http://localhost:8080/objects?collection=users' \
  -H 'Content-Type: application/json' \
  -d '{"key":"u:1","value":{"name":"Sara"}}'

# 2) دریافت
curl 'http://localhost:8080/objects/u:1?collection=users'

# 3) لیست
curl 'http://localhost:8080/objects?collection=users&prefix=u:&limit=2&offset=0'
```

---

**یادداشت استادانه:** اگر خواستید برای نمرهٔ بیشتر، یک endpoint سادهٔ `POST /admin/compact?collection=...` هم می‌توانید اضافه کنید تا دستی compaction اجرا شود. این نسخه compaction را خودکار و محافظه‌کار انجام می‌دهد.
