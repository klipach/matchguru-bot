## Deploy
```bash
firebase deploy --only hosting
```


```mermaid
flowchart TB
    A((Start)) --> B{External knowledge required <br>(ChatGPT)?}
    B -- Yes --> C[Request data perplexity]
    C --> D[Reply to customer <br>including perplexity response (ChatGPT)]
    B -- No --> D
    D((End))
```
