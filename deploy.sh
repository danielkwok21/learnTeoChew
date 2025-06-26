cd ~/learnTeochew

if [ ! -f "cronhistory.txt" ]; then
    touch cronhistory.txt
    echo "cronhistory.txt created."
else
    echo "cronhistory.txt already exists."
fi

# rm logfile if it's too big. All outputs will be populated here
filesize=$(wc -l < cronhistory.txt)
if [ $filesize -gt 1000 ]; then
  rm cronhistory.txt
fi

echo "\n***RAN ON:$(date)***"
# Fetch the latest changes from origin
git fetch origin master

# Get the latest commit hash on remote and local master
remote_commit=$(git rev-parse origin/master)
local_commit=$(git rev-parse HEAD)

if [ "$remote_commit" != "$local_commit" ]; then
    echo "New commit detected. Pulling and rebuilding..."
    git pull origin master
    go build -o main main.go
    nohup ./main > app.log 2>&1 &
    echo "App rebuilt and running in background."
else
    echo "No new commits on master branch."
fi