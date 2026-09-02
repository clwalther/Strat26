import json
import shutil
import datetime
import threading
from http.server import ThreadingHTTPServer
from http.server import SimpleHTTPRequestHandler

from database import Database

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

		else:
			self.send_error(404)

def start(time=None, call_index = 0):
	global timer

	now = datetime.datetime.now(datetime.timezone.utc)
	dt = (now - time).total_seconds() if time is not None else 0
	call_index += 1

	internal_update(dt, call_index)

	# init timer
	timer = threading.Timer(INTERVAL, start, [now, call_index])
	timer.start()

def stop():
	global timer

	timer.cancel()

def internal_update(dt, call_index):
	print(f"#{call_index} dt={dt}")


if __name__ == "__main__":
	INTERVAL = 10 # sec

	timer = None
	db = Database("./database/database.db")
	fs = ThreadingHTTPServer(("0.0.0.0", 8080), Handler)

	print("Server running at http://localhost:8080")

	try:
		fs.serve_forever()
	except ConnectionAbortedError:
		print("Connection aborted...")
	except KeyboardInterrupt:
		fs.server_close()
		db.shutdown()
