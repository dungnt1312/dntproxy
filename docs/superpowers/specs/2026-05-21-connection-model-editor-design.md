# Connection Model Editor Design

## Goal

Make connection model management fast when a provider exposes many models. The main workflow should let users edit a newline-separated list, paste large model lists, and save without clicking many checkboxes.

## Scope

- Replace the connection `EditModelsModal` checklist workflow with a textarea-first editor.
- Keep the existing backend storage model: `ProviderConnection.SupportedModels` remains a whitelist, and an empty list means all models are allowed.
- Add a small remove action in the routing model table for connections that already use a restricted model list.
- Do not add a denylist or new backend endpoint in this iteration.

## Modal Behavior

When opened from the Connections screen, the modal shows a large textarea where each line is one model ID.

- If `supportedModels` has values, the textarea is populated with those values, one per line.
- If `supportedModels` is empty, the textarea starts empty and the modal clearly states that all models are allowed.
- Saving splits by line, trims whitespace, removes blank lines, deduplicates while preserving order, and sends `supportedModels` with `setModels: true` to `PUT /api/connections/:id`.
- Clearing the textarea and saving returns the connection to all-models-allowed mode.

The modal includes lightweight actions:

- `Fetch from API`: replaces the textarea with fetched model IDs, one per line, and shows that saving is still required.
- `Clear`: clears the textarea.
- `Copy`: copies the current textarea content for quick external editing.

## Routing Models Table Behavior

The Models tab keeps showing each model and its available connections. Each connection chip gets a remove action.

- If the connection has a restricted `supportedModels` list, remove deletes that model from the list and saves the updated connection.
- If the connection allows all models, remove does not silently convert semantics. The UI tells the user that the connection currently allows all models and directs them to open the model editor to create a restricted list.

## Data Flow

- UI continues to load connections from `GET /api/connections` and models from `GET /api/models`.
- Model editor saves through the existing `api.updateConnection(conn.id, { supportedModels, setModels: true })` path.
- The Models tab removal action needs access to current connection `supportedModels`, so it should load or receive connection data in addition to model rows.
- After any save, reload models/connections so counts and connection chips stay accurate.

## Error Handling

- Show toast errors for fetch, save, copy, and remove failures.
- Keep modal content intact when save fails so the user does not lose pasted edits.
- When fetch fails, leave the textarea unchanged.
- For allow-all removal attempts, do not change data; show a clear message and optionally open the editor.

## Testing

- Unit-test parsing helper if extracted: trims, removes blank lines, and deduplicates preserving order.
- Manually verify the connection modal with restricted and allow-all connections.
- Manually verify fetch-replace-save flow.
- Manually verify removing a model from a restricted connection in the Models tab.
- Manually verify allow-all removal does not modify connection data.
