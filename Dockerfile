FROM golang:1.26-alpine AS build
WORKDIR /src
COPY go.mod main.go api.go strategy.go duel_search.go deep_search.go strategy_experiments.go constrictor_route.go constrictor_rollout.go ./
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /battlesnake .

FROM scratch
COPY --from=build /battlesnake /battlesnake
USER 65532:65532
ENV PORT=8080 GOMAXPROCS=1
EXPOSE 8080
ENTRYPOINT ["/battlesnake"]
