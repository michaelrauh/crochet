```mermaid
sequenceDiagram
    participant Worker
    participant Repository
    participant DB 

    Worker->>Repository: GET /Context
    Repository->>DB: Read(Context)
    Repository->>Worker: Reply (200, Context) 
```   