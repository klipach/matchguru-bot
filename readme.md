## Deploy
```bash
firebase deploy --only hosting
```



## Supported HTML tags in the message
Bot support the same set of tags as Telegram API (source: https://help.publer.io/en/article/how-to-style-telegram-text-using-html-tags-xdepnw/)
```
<b>bold</b>, <strong>bold</strong>
<i>italic</i>, <em>italic</em>
<u>underline</u>, <ins>underline</ins>
<s>strikethrough</s>, <strike>strikethrough</strike>, <del>strikethrough</del>
<span class="tg-spoiler">spoiler</span>, <tg-spoiler>spoiler</tg-spoiler>
<b>bold <i>italic bold <s>italic bold strikethrough <span class="tg-spoiler">italic bold strikethrough spoiler</span></s> <u>underline italic bold</u></i> bold</b>
<a href="http://www.example.com/">inline URL</a>
<code>inline fixed-width code</code>
<pre>pre-formatted fixed-width code block</pre>
<pre><code class="language-python">pre-formatted fixed-width code block written in the Python programming language</code></pre>
```

## Main flow
```mermaid
flowchart TB
    A((Start)) --> B{External knowledge required <br>(ChatGPT)?}
    B -- Yes --> C[Request data perplexity]
    C --> D[Reply to customer <br>including perplexity response (ChatGPT)]
    B -- No --> D
    D((End))
```
