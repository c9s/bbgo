# Google Spreadsheet Integration

## Setup

Run gcloud init to setup a new profile:

```shell
gcloud init
```

Enable Sheet API:

```shell
gcloud services enable sheets.googleapis.com
```

You will need to setup a service account for your bbgo application, 
check the following documentation to setup the authentication:

<https://developers.google.com/identity/protocols/oauth2/service-account>

And 

<https://developers.google.com/workspace/guides/create-credentials>

Download the JSON token file and store it in a safe place.

### Setting up service account permissions

Go to Google Workspace and Add "Manage Domain Wide Delegation", add you client and with the following scopes:

```
https://www.googleapis.com/auth/drive
https://www.googleapis.com/auth/drive.file
https://www.googleapis.com/auth/drive.readonly
https://www.googleapis.com/auth/spreadsheets
https://www.googleapis.com/auth/spreadsheets.readonly
```


### Add settings to your bbgo.yaml

```shell
services:
  googleSpreadSheet:
    jsonTokenFile: ".credentials/google-cloud/service-account-json-token.json"
    spreadSheetId: "YOUR_SPREADSHEET_ID"
```
