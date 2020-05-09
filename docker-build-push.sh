GOOS=linux go build -o bin/k8-exercise
docker build -t davidrthorn/kube-exercise-server:latest .
docker push davidrthorn/kube-exercise-server:latest
