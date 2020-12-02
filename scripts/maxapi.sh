#!/bin/bash
base_url="https://max-api.maicoin.com"


# server use seconds
ts=$(date "+%s")
server_ts=$(curl -s $base_url/api/v2/timestamp)
offset=$(( $server_ts - $ts ))

function nonce()
{
    local ts=$(( $(gdate "+%s%N") / 1000000 ))
    local mts=$(($ts + $offset * 1000))
    echo $mts
}

function generate_payload()
{
    local req_path=$1
    local -n ref_params=$2

    declare -A payload
    payload[nonce]=$(nonce)
    payload[path]=$req_path

    for k in "${!ref_params[@]}"
    do
        payload[$k]=${ref_params[$k]}
    done

    for k in "${!payload[@]}"
    do
        echo "\"$k\""
        echo "\"${payload[$k]}\""
    done | jq -c -n 'reduce inputs as $i ({}; . + { ($i): (input|(tonumber? // .)) })'
}

function hmac_sha1()
{
    local value=$1
    local key=$2
    echo -n "$value" | openssl dgst -sha256 -hmac "$key" | sed -e 's/^.* //'
}

function send_auth_request()
{
    local req_method=$1
    local req_path=$2
    local -n req_params=$3
    payload_json=$(generate_payload $req_path req_params)

    # echo "payload json: $payload_json"

    encoded_payload=$(echo -n "$payload_json" | base64)

    # echo "encoded payload: $encoded_payload"

    payload_sig=$(hmac_sha1 "$encoded_payload" "$MAX_API_SECRET")

    # echo "signature: $payload_sig with $MAX_API_KEY/$MAX_API_SECRET"

    curl -s -X $req_method -H "Content-Type: application/json" \
        -H "Accept: application/json" \
        -H "X-MAX-ACCESSKEY: $MAX_API_KEY" \
        -H "X-MAX-PAYLOAD: $encoded_payload" \
        -H "X-MAX-SIGNATURE: $payload_sig" \
        --data "$payload_json" \
        "${base_url}${req_path}" | jq
}

function send_oauth_request()
{
    local req_method=$1
    local req_path=$2
    local -n req_params=$3
    payload_json=$(generate_payload $req_path req_params)
    encoded_payload=$(echo -n "$payload_json" | base64)
    payload_sig=$(hmac_sha1 "$encoded_payload" "$MAX_API_SECRET")
    curl -s -X $req_method -H "Content-Type: application/json" \
        -H "Accept: application/json" \
        -H "X-MAX-ACCESSKEY: $MAX_API_KEY" \
        -H "X-MAX-PAYLOAD: $encoded_payload" \
        -H "X-MAX-SIGNATURE: $payload_sig" \
        -H "Client-Id: $MAX_CLIENT_ID" \
        -H "Client-Secret: $MAX_CLIENT_SECRET" \
        --data "$payload_json" \
        "${base_url}${req_path}" | jq
}

function me()
{
    declare -A params=()
    send_auth_request "GET" "/api/v2/members/me" params
}

function submitOrder()
{
    local -n params=$1
    send_auth_request "POST" "/api/v2/orders" params
}

function cancelOrder()
{
    declare -A params=()
    params[id]=$1
    send_auth_request "POST" "/api/v2/order/delete" params
}

function myOrders()
{
    local -n params=$1
    send_auth_request "GET" "/api/v2/orders" params
}


function myTrades()
{
    local -n params=$1
    send_auth_request "GET" "/api/v2/trades/my" params
}


# me | jq '.accounts[] | select(.currency == "usdt")'
# me | jq '.accounts[] | select(.currency == "btc")'
# me | jq '.accounts[] | select(.currency == "eth")'
# me

# declare -A order_params=()
# order_params[market]="ethusdt"
# order_params[side]="buy"
# order_params[volume]="0.05"
# # order_params[price]="585"
# order_params[ord_type]="market"
# submitOrder order_params

# declare -A my_trades_params=()
# my_trades_params[market]="ethusdt"
# myTrades my_trades_params

# declare -A my_orders_params=()
# my_orders_params[market]="ethusdt"
# myOrders my_orders_params

