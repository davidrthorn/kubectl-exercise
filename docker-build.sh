GOOS=linux go build -o bin/rio
docker build -t davidrthorn/kube-exercise-server:latest .
docker push davidrthorn/kube-exercise-server:latest
