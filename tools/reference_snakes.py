"""Deterministic opponents for local Battlesnake benchmarks."""

import argparse
from collections import deque
from http.server import BaseHTTPRequestHandler, ThreadingHTTPServer
import json
import socket


DIRECTIONS = (
    ("up", 0, 1),
    ("right", 1, 0),
    ("down", 0, -1),
    ("left", -1, 0),
)


def point(cell):
    return cell["x"], cell["y"]


def neighbors(cell):
    for _, dx, dy in DIRECTIONS:
        yield cell[0] + dx, cell[1] + dy


def reachable(start, available):
    seen = {start}
    queue = deque([start])
    while queue:
        for cell in neighbors(queue.popleft()):
            if cell in available and cell not in seen:
                seen.add(cell)
                queue.append(cell)
    return len(seen)


def move(request, policy):
    board = request["board"]
    you = request["you"]
    rules = request["game"]["ruleset"]
    constrictor = rules["name"] == "constrictor"
    width, height = board["width"], board["height"]
    food = set() if constrictor else {point(cell) for cell in board["food"]}
    hazards = {point(cell) for cell in board["hazards"]}
    damage = rules.get("settings", {}).get("hazardDamagePerTurn", 14)
    head = point(you["head"])
    occupied = set()
    for snake in board["snakes"]:
        body = snake["body"]
        occupied.update(map(point, body if constrictor else body[:-1]))
    available = {(x, y) for x in range(width) for y in range(height)} - occupied
    contested = set()
    for snake in board["snakes"]:
        if snake["id"] != you["id"] and snake["length"] >= you["length"]:
            contested.update(neighbors(point(snake["head"])))

    best_name = "up"
    best_score = float("-inf")
    for name, cell in zip((item[0] for item in DIRECTIONS), neighbors(head)):
        if cell not in available:
            continue
        hazard = damage if cell in hazards and cell not in food else 0
        if not constrictor and cell not in food and you["health"] <= 1 + hazard:
            continue
        distance = min((abs(cell[0] - x) + abs(cell[1] - y) for x, y in food), default=0)
        escape = sum(adjacent in available for adjacent in neighbors(cell))
        area = reachable(cell, available)
        center = abs(2 * cell[0] - width + 1) + abs(2 * cell[1] - height + 1)
        if policy == "greedy":
            score = -10 * distance + escape
        elif policy == "space":
            score = area + 3 * escape - distance
        else:
            score = 0.45 * area + 4 * escape - 2 * distance - center
        if policy != "greedy" and cell in contested:
            score -= 1_000
        score -= hazard * 2
        if score > best_score:
            best_name, best_score = name, score
    return best_name


class Handler(BaseHTTPRequestHandler):
    protocol_version = "HTTP/1.1"

    def respond(self, status, payload=None):
        content = b"" if payload is None else json.dumps(payload).encode()
        self.send_response(status)
        self.send_header("Content-Length", str(len(content)))
        self.send_header("Content-Type", "application/json")
        self.end_headers()
        self.wfile.write(content)

    def do_GET(self):
        self.respond(200, {"apiversion": "1"})

    def do_POST(self):
        self.connection.setsockopt(socket.IPPROTO_TCP, socket.TCP_NODELAY, 1)
        body = self.rfile.read(int(self.headers.get("Content-Length", "0")))
        parts = self.path.strip("/").split("/")
        if parts[-1] in ("start", "end"):
            self.respond(200)
        elif len(parts) == 2 and parts[0] in ("greedy", "space", "center") and parts[1] == "move":
            self.respond(200, {"move": move(json.loads(body), parts[0])})
        else:
            self.respond(404)

    def log_message(self, format, *args):
        pass


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--port", type=int, default=8081)
    args = parser.parse_args()
    with ThreadingHTTPServer(("127.0.0.1", args.port), Handler) as server:
        server.serve_forever()


if __name__ == "__main__":
    main()
