# syntax=docker/dockerfile:1

# ==========================================
# အဆင့် ၁ — Build (Compilation)
# ==========================================
FROM golang:1.23-alpine AS builder

# Build အတွက် လိုအပ်တဲ့ tools များ (Git, Certs)
RUN apk add --no-cache git ca-certificates tzdata

WORKDIR /src

# Dependency တွေကို အရင်ကူးယူပြီး Cache လုပ်မယ် (Build မြန်စေရန်)
COPY go.mod go.sum ./
RUN go mod download && go mod verify

# ကျန်တဲ့ ကုဒ်တွေ အားလုံး ကူးယူ
COPY . .

# Version အချက်အလက်များကို ထည့်သွင်းရန် Arguments များ
ARG VERSION=dev
ARG COMMIT=none
ARG DATE=unknown

# Static Binary အနေနဲ့ Build လုပ်မယ်
RUN CGO_ENABLED=0 GOOS=linux go build \
    -trimpath \
    -ldflags="-s -w \
        -X 'tg-dlp/pkg/runtime.Version=${VERSION}' \
        -X 'tg-dlp/pkg/runtime.Commit=${COMMIT}' \
        -X 'tg-dlp/pkg/runtime.BuildTime=${DATE}'" \
    -o /out/tg-dlp .

# ==========================================
# အဆင့် ၂ — Run (Runtime)
# ==========================================
FROM alpine:3.20

# အလိုအလျောက် Update ဖြစ်ဖို့နဲ့ Telegram (HTTPS) အတွက် CA Certificates လိုအပ်
RUN apk add --no-cache ca-certificates tzdata

# လုံခြုံရေးအတွက် Root မဟုတ်တဲ့ User အသစ်တစ်ခု ဖန်တီး
RUN addgroup -S tgdlp && adduser -S -G tgdlp tgdlp

# Build လုပ်ထားတဲ့ binary ကို ကူးယူ
COPY --from=builder /out/tg-dlp /usr/local/bin/tg-dlp

# User ပြောင်းလဲ
USER tgdlp
WORKDIR /home/tgdlp

# Session နဲ့ Config ဖိုင်တွေ သိမ်းဆည်းဖို့ Volume တစ်ခု သတ်မှတ်
# (ဒါမှ Docker container ပြန်ဖျက်လည်း session မပျက်သွားအောင်)
VOLUME ["/home/tgdlp/.tg-dlp"]

# CLI အတွက် ENTRYPOINT
ENTRYPOINT ["tg-dlp"]
CMD ["help"]