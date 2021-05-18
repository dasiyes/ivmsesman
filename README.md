# IVMSESMAN
IVMSESMAN is package to manage sessions

## Description
Complete functionality should include session storage management. Cretion of unique session tokens.
Manage sessions time-outs. Clean inactive and expired sessions.

## v0.1.2

Implements as of now the session store with INMEM provider.
FireStore and Redis to be implemented in the coming versions.

Time-outed session cleaning to be tested.

## Firestore as Session Store provider

If the Session Manager will be used with Firestore as session store provider, there MUST be a env variable to identify the GCP projectID for the firestore. This will be read at init time of the session manager.
