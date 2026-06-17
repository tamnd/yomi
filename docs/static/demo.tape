# yomi demo. ascii-gif prepends the window chrome, theme, and framing;
# this tape carries only the action.
Set Width 1200
Set Height 700

Type "# A web page, read straight into clean Markdown" Sleep 600ms Enter
Type "yomi read paulgraham.com/greatwork.html | head -n 11" Sleep 800ms Enter
Sleep 3.5s

Type "# Save it to a file, get a friendly one-line summary" Sleep 600ms Enter
Type "yomi read paulgraham.com/greatwork.html -o greatwork.md" Sleep 800ms Enter
Sleep 2.5s

Type "# Just the metadata, as JSON you can pipe into jq" Sleep 600ms Enter
Type "yomi meta paulgraham.com/greatwork.html | jq '{title, word_count, reading_time}'" Sleep 800ms Enter
Sleep 3.5s
