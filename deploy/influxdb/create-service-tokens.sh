#!/bin/sh

set -eu

token_directory=/tokens
checker_token_file="$token_directory/checker-token"
metrics_token_file="$token_directory/metrics-token"

if [ -s "$checker_token_file" ] && [ -s "$metrics_token_file" ]; then
    exit 0
fi

bucket_json=$(influx bucket list \
    --host "$INFLUX_HOST" \
    --org "$INFLUX_ORG" \
    --token "$INFLUX_TOKEN" \
    --name "$INFLUX_BUCKET" \
    --json)
bucket_id=$(printf '%s' "$bucket_json" |
    sed -n 's/.*"id":[[:space:]]*"\([^"]*\)".*/\1/p')

if [ -z "$bucket_id" ]; then
    echo "Could not resolve InfluxDB bucket ID" >&2
    exit 1
fi

mkdir -p "$token_directory"

create_token() {
    permission=$1
    description=$2
    destination=$3

    authorization=$(influx auth create \
        --host "$INFLUX_HOST" \
        --org "$INFLUX_ORG" \
        --token "$INFLUX_TOKEN" \
        "$permission" "$bucket_id" \
        --description "$description" \
        --json)
    service_token=$(printf '%s' "$authorization" |
        sed -n 's/.*"token":[[:space:]]*"\([^"]*\)".*/\1/p')

    if [ -z "$service_token" ]; then
        echo "Could not create $description" >&2
        exit 1
    fi

    printf '%s\n' "$service_token" > "$destination"
    chmod 0444 "$destination"
}

create_token --write-bucket checker-write "$checker_token_file"
create_token --read-bucket metrics-read "$metrics_token_file"
