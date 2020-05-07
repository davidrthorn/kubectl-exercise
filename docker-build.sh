GOOS=linux go build -o bin/server
docker build -t kube-exercise-server .
