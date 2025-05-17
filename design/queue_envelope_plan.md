# Plan: Normalized Envelope Publishing for Handler Methods

## Goals

- Expose four handler methods to publish:
  1. Vocabulary
  2. Subphrases
  3. Start sigil
  4. End sigil
- All queue messages use a single envelope type: `{Type, Data}`.
- `Data` is one of four predefined structs (one per message type).
- Envelope is serialized (e.g., JSON) to a byte array for the queue.
- All messages are published to the `"db"` channel (queue). There will be exactly one other channel added later.
- **Envelope and payload types must be defined in a shared package (e.g., `pkg/queueenvelope/types.go`) so all services can import them.**
- Ensure testability and compatibility with existing tests.

## Steps

### 1. Define Envelope and Data Types

- Create an `Envelope` struct with fields:
  - `Type` (string): e.g., "Vocabulary", "Subphrases", "StartSigil", "EndSigil"
  - `Data` (json.RawMessage or interface{}): the payload
- Define four structs for the payloads:
  - `VocabularyPayload` (e.g., `Words []string`)
  - `SubphrasesPayload` (e.g., `Phrases [][]string`)
  - `StartSigilPayload` (e.g., `Sigil string`)
  - `EndSigilPayload` (e.g., `Sigil string`)
- **Place these types in a new shared file: `/Users/michaelrauh/dev/crochet/pkg/queueenvelope/types.go`**

### 2. Serialization

- Use JSON to marshal the envelope to a byte array before publishing to the queue.

### 3. Handler Methods

- Implement four handler methods, each:
  - Accepts the relevant data (vocabulary, subphrases, start sigil, end sigil).
  - Wraps the data in the appropriate payload struct.
  - Wraps the payload in an envelope with the correct type.
  - Marshals the envelope to JSON.
  - Publishes the byte array to the `"db"` queue.

### 4. Queue Integration

- Use the existing `Publish` method, passing the marshaled envelope as the body and `"db"` as the queue name.

### 5. Testing

- Unit test each handler method:
  - Verify correct envelope construction and serialization.
  - Mock the queue to assert the correct byte array is published.
- Integration test:
  - End-to-end test to ensure the queue receives the correct envelope.

### 6. Backward Compatibility

- None needed 

### 7. Documentation

- None needed

---

## Example Envelope

```json
{
  "Type": "Vocabulary",
  "Data": {
    "Words": ["hello", "world"]
  }
}
```

---

## Next Steps

1. Define the envelope and payload structs in `/Users/michaelrauh/dev/crochet/pkg/queueenvelope/types.go`.
2. Implement the four handler methods.
3. Update tests to cover the new logic.
4. Update queue consumers if needed.
