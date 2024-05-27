import os

import msal

CLIENT_ID = os.getenv("ONE_DRIVE_CLIENT_ID", "")
CLIENT_SECRET = os.getenv("ONE_DRIVE_CLIENT_SECRET", "")


def acquire_token_func() -> dict:
    if not CLIENT_ID or not CLIENT_SECRET:
        raise ValueError(
            "Could not find required environment variables "
            "to authenticate with OneDrive."
        )

    authority_url = "https://login.microsoftonline.com/common"
    app = msal.ConfidentialClientApplication(
        authority=authority_url,
        client_id=f"{CLIENT_ID}",
        client_credential=f"{CLIENT_SECRET}",
    )
    token = app.acquire_token_for_client(
        scopes=["https://graph.microsoft.com/.default"]
    )
    if not isinstance(token, dict):
        raise ValueError("Could not acquire token")

    return token
