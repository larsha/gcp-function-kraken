# function.go

Google Cloud Function using Go 1.11 with modules.
This function triggers on `google.storage.object.finalize` event and sends the file to [Kraken.io](https://kraken.io/) to compress/optimize it then replaces the file in the bucket.

This function should be deployed where you want a decoupled image compression tool to react when images are saved to Google Cloud Storage in your application.

```
export BUCKET=""
export KRAKEN_API_KEY=""
export KRAKEN_SECRET_KEY=""
```

```
gcloud functions deploy ImageCompressor \
    --set-env-vars KRAKEN_API_KEY=$KRAKEN_API_KEY,KRAKEN_SECRET_KEY=$KRAKEN_SECRET_KEY \
    --region europe-west1 \
    --runtime go113 \
    --trigger-resource $BUCKET \
    --trigger-event google.storage.object.finalize
```
