```mermaid
sequenceDiagram
    Actor User 
    participant User
    participant Repository
    participant DB Queue

    User->>Repository: POST /Corpora (Title, Text)
    Repository->>DB Queue: Push(Start_Sigil)
    Repository->>DB Queue: Push(Context)
    Repository->>DB Queue: Push(End_Sigil)
    Repository->>DB Queue: Push(Seed)
    Repository->>User: Reply (202)  
``` 