# Project Changelog

## 2026-05-12

- Added dashboard API key permission management for connection and model allowlists.
- Added API client support for `allowedConnectionIds`, `allowedModels`, and key updates.
- Added backend API key handler validation/deduplication for allowed connection IDs.
- Added handler tests for API key permission create/update validation.
- Fixed `/v1/models` so API key connection/model allowlists also filter direct models, aliases, and combos.
