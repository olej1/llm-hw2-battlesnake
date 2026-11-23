FROM rust:1-alpine AS build
RUN apk add --no-cache musl-dev
WORKDIR /src
COPY Cargo.toml ./
COPY src ./src
COPY benches ./benches
RUN cargo build --release --bin server

FROM alpine:3.22
COPY --from=build /src/target/release/server /snork
USER 65532:65532
ENTRYPOINT ["/snork","--host","0.0.0.0:5001"]
