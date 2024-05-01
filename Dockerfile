FROM golang:latest

WORKDIR /app

COPY go.mod ./

RUN go mod download

COPY . .

RUN go build -o main

EXPOSE 9899

CMD ["./main"]
