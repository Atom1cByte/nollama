import json

count = 0
with open("myresults.json", "r") as f:
    for line in f:
        line = line.strip()
        if line:
            try:
                json.loads(line)
                count += 1
            except json.JSONDecodeError:
                pass

print(f"Total results: {count}")
