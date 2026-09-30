#!/usr/bin/env bash
set -e

SERVER="${TINK_SERVER:-http://localhost:5021}"
TOKEN="${TINK_TOKEN:-}"
TITLE=""
BODY=""
URL=""
SOUND="default"
GROUP=""
DEVICES=""

while [[ $# -gt 0 ]]; do
  case $1 in
    --server | -s)
      SERVER="$2"
      shift 2
      ;;
    --token | -t)
      TOKEN="$2"
      shift 2
      ;;
    --title)
      TITLE="$2"
      shift 2
      ;;
    --body | -b)
      BODY="$2"
      shift 2
      ;;
    --url | -u)
      URL="$2"
      shift 2
      ;;
    --sound)
      SOUND="$2"
      shift 2
      ;;
    --group | -g)
      GROUP="$2"
      shift 2
      ;;
    --devices | -d)
      DEVICES="$2"
      shift 2
      ;;
    *)
      if [ -z "$TITLE" ]; then
        TITLE="$1"
      elif [ -z "$BODY" ]; then
        BODY="$1"
      fi
      shift
      ;;
  esac
done

if [ -z "$TITLE" ] && [ -z "$BODY" ]; then
  TITLE="Tink Notification"
  BODY="Test message sent at $(date '+%Y-%m-%d %H:%M:%S')"
fi

PAYLOAD=$(
  cat << JSON
{
  "title": $(printf '%s' "$TITLE" | python3 -c 'import json,sys; print(json.dumps(sys.stdin.read()))'),
  "body": $(printf '%s' "$BODY" | python3 -c 'import json,sys; print(json.dumps(sys.stdin.read()))'),
  "url": $(printf '%s' "$URL" | python3 -c 'import json,sys; print(json.dumps(sys.stdin.read()))'),
  "sound": $(printf '%s' "$SOUND" | python3 -c 'import json,sys; print(json.dumps(sys.stdin.read()))'),
  "group": $(printf '%s' "$GROUP" | python3 -c 'import json,sys; print(json.dumps(sys.stdin.read()))')
}
JSON
)

AUTH_HEADER=()
if [ -n "$TOKEN" ]; then
  AUTH_HEADER=(-H "Authorization: Bearer $TOKEN")
fi

echo "==> Sending notification to $SERVER/api/v1/messages..."
RESPONSE=$(curl -s -w "\n%{http_code}" -X POST "$SERVER/api/v1/messages" \
  "${AUTH_HEADER[@]}" \
  -H "Content-Type: application/json" \
  -d "$PAYLOAD")

HTTP_CODE=$(echo "$RESPONSE" | tail -n1)
BODY_RESP=$(echo "$RESPONSE" | sed '$d')

if [ "$HTTP_CODE" -ge 200 ] && [ "$HTTP_CODE" -lt 300 ]; then
  echo "✓ Notification sent successfully (HTTP $HTTP_CODE)!"
  echo "$BODY_RESP"
else
  echo "✗ Failed to send notification (HTTP $HTTP_CODE):"
  echo "$BODY_RESP"
  exit 1
fi
