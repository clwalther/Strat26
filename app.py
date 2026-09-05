import json
import shutil
import datetime
import threading
import random
import math
from http.server import ThreadingHTTPServer
from http.server import SimpleHTTPRequestHandler

from database import Database
from utils import *

class Handler(SimpleHTTPRequestHandler):
	def __init__(self, *args, **kwargs):
		super().__init__(*args, directory="./source", **kwargs)

	def do_GET(self):
		# API
		if self.path == "/api/status":
			status = db.get_status()
			duration = db.get_duration()
			version = db.get_version()

			if (status != -1
					and duration != -1
					and version != -1):
				response = json.dumps({
					"status": status,
					"duration": duration,
					"version": f"SQLite {version}"
				}).encode("utf-8")

				self.send_response(200)
				self.send_header("Content-Type", "application/json")
				self.send_header("Content-Length", len(response))
				self.end_headers()
				self.wfile.write(response)

			else:
				self.send_error(500, "Database Problems")

		elif self.path == "/api/players":
			players = db.get_players()

			if players != -1:
				response = json.dumps({
					"players": players,
				}).encode("utf-8")

				self.send_response(200)
				self.send_header("Content-Type", "application/json")
				self.send_header("Content-Length", len(response))
				self.end_headers()
				self.wfile.write(response)
			else:
				self.send_error(500, "Database Problems")

		elif self.path == "/api/score":
			score = db.get_score()
			momentum = db.get_momentum()

			if momentum != -1:
				response = json.dumps({
					"score": score,
					"momentum": momentum,
				}).encode("utf-8")

				self.send_response(200)
				self.send_header("Content-Type", "application/json")
				self.send_header("Content-Length", len(response))
				self.end_headers()
				self.wfile.write(response)
			else:
				self.send_error(500, "Database Problems")

		# Fileserver
		elif self.path == "/":
			self.path = "/index.html"
			super().do_GET()

		elif self.path == "/dashboard" or self.path == "/dashboard/":
			self.path = "/dashboard.html"
			super().do_GET()

		elif self.path == "/interface-blue" or self.path == "/interface-blue/":
			self.path = "/interface-blue.html"
			super().do_GET()

		elif self.path == "/interface-green" or self.path == "/interface-green/":
			self.path = "/interface-green.html"
			super().do_GET()

		else:
			super().do_GET()

	def do_POST(self):
		if self.path == "/api/status":
			length = int(self.headers.get("Content-Length", 0))
			body = self.rfile.read(length)

			try:
				request = json.loads(body)
				status = request.get("status")

				curr = db.get_status()
				succ = db.set_status(status)

				if succ == 0 and curr != -1:
					if curr != status and status == "RUNNING":
						start()

					elif curr != status and status == "PAUSED":
						stop()

					self.send_response(200)
					self.end_headers()
				else:
					self.send_error(500, "Database Problems")

			except json.JSONDecodeError:
				self.send_error(400, "Invalid JSON")

		elif self.path == "/api/backup":
			filename = datetime.datetime.now().strftime("./database/%Y-%m-%d %H.%M.%S.%f.db")

			shutil.copy("./database/database.db", filename)
			print(f"Backed up the database: {filename}")

			self.send_response(200)
			self.end_headers()

		elif self.path == "/api/positions":
			length = int(self.headers.get("Content-Length", 0))
			body = self.rfile.read(length)

			try:
				request = json.loads(body)
				player = request.get("players")

				succ = db.set_positions(player)

				if succ == 0:
					self.send_response(200)
					self.end_headers()
				else:
					self.send_error(500, "Database Problems")

			except json.JSONDecodeError:
				self.send_error(400, "Invalid JSON")

		elif self.path == "/api/player":
			length = int(self.headers.get("Content-Length", 0))
			body = self.rfile.read(length)

			try:
				request = json.loads(body)
				player = request.get("player")

				succ = db.set_player(player)

				if succ == 0:
					self.send_response(200)
					self.end_headers()
				else:
					self.send_error(500, "Database Problems")

			except json.JSONDecodeError:
				self.send_error(400, "Invalid JSON")

		elif self.path == "/api/punishment":
			length = int(self.headers.get("Content-Length", 0))
			body = self.rfile.read(length)

			try:
				request = json.loads(body)
				player = request.get("player")

				succ = db.set_punishment(player)

				if succ == 0:
					self.send_response(200)
					self.end_headers()
				else:
					self.send_error(500, "Database Problems")

			except json.JSONDecodeError:
				self.send_error(400, "Invalid JSON")

		elif self.path == '/api/score':
			length = int(self.headers.get("Content-Length", 0))
			body = self.rfile.read(length)

			try:
				request = json.loads(body)
				score = request.get("score")
				team = request.get("team")

				if team == 'blau':
					succ = db.set_score_blue(score)

					if succ == 0:
						self.send_response(200)
						self.end_headers()
					else:
						self.send_error(500, "Database Problems")
				elif team == 'grün':
					succ = db.set_score_green(score)

					if succ == 0:
						self.send_response(200)
						self.end_headers()
					else:
						self.send_error(500, "Database Problems")
				else:
					self.send_error(300, 'Invalid data given to the server')

			except json.JSONDecodeError:
				self.send_error(400, "Invalid JSON")

		else:
			self.send_error(404)

def start(time=None, call_index = 0):
	global timer

	status = db.get_status()

	if status != "PAUSED":
		now = datetime.datetime.now(datetime.timezone.utc)
		dt = (now - time).total_seconds() if time is not None else 0
		call_index += 1

		worker(dt, call_index)

		# init timer
		timer = threading.Timer(INTERVAL, start, [now, call_index])
		timer.start()

def stop():
	global timer

	timer.cancel()

def worker(dt, i):
	update_fatigue(dt)
	update_momentum(dt)
	create_chance(dt)

# === game mechanics ===
def update_fatigue(dt):
	players = db.get_players()

	for player in players:
		if player['role'] == 'BENCH':
			fatigue = player['fatigue'] - FATIGUE * dt
			db.set_fatigue(max(fatigue, 0), player['id'])

		else:
			fatigue = player['fatigue'] + 2*random.random() * FATIGUE * dt
			db.set_fatigue(min(fatigue, 100), player['id'])

def update_momentum(dt):
	momentum = db.get_momentum()
	players = db.get_players()

	eval_blue  = get_eval("blau", players)
	eval_green = get_eval("grün", players)

	balance = eval_green / (eval_blue + eval_green)

	print(
		round(eval_green),
		round(eval_blue),
		f"{round(balance*100)}/{100-round(balance*100)}"
	)

	if random.random() < balance:
		momentum += STEP * dt
	else:
		momentum -= STEP * dt

	momentum = max(-100, momentum)
	momentum = min(momentum, 100)

	db.set_momentum(momentum)

def create_chance(dt):
	momentum = db.get_momentum()
	score = db.get_score()

	if momentum > 70:
		# green goal
		if random.random() < PROBABILITY:
			db.set_score_blue(score["grün"]+1)
			db.set_momentum(0)

	if momentum < -70:
		# blue goal
		if random.random() < PROBABILITY:
			db.set_score_blue(score["blau"]+1)
			db.set_momentum(0)


if __name__ == "__main__":
	INTERVAL    = 5 / 5					# sec
	FATIGUE     = (100 / 25) / 60 * 5	# fatigue / sec
	STEP        = 1 * 5				# momentum / sec
	PROBABILITY = 1 / 10 * 5			# probability / sec

	timer = None
	db = Database("./database/database.db")
	fs = ThreadingHTTPServer(("0.0.0.0", 8080), Handler)

	print("Server running at http://localhost:8080")

	try:
		fs.serve_forever()
	except KeyboardInterrupt:
		fs.server_close()
		db.shutdown()
