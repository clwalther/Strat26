import json
import shutil
import datetime
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

			if (status is not None
					and duration is not None
					and version is not None):
				response = json.dumps({
					"status": status,
					"duration": duration,
					"version": version
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

			if players is not None:
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

		else:
			super().do_GET()

	def do_POST(self):
		if self.path == "/api/status":
			length = int(self.headers.get("Content-Length", 0))
			body = self.rfile.read(length)

			try:
				request = json.loads(body)
				status = request.get("status")

				succ = db.set_status(status)

				if succ == 0:
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

		else:
			self.send_error(404)



if __name__ == "__main__":
	db = Database("./database/database.db")
	fs = ThreadingHTTPServer(("0.0.0.0", 8080), Handler)

	# # ===========
	# con = db.connection()
	# cur = con.cursor()

	# cur.execute("SELECT * FROM timetable;")
	# for row in cur.fetchall():
	# 	print(row)
	# cur.execute("SELECT * FROM players;")
	# for row in cur.fetchall():
	# 	print(row)

	# con.commit()
	# con.close()
	# # ===========

	print("Server running at http://localhost:8080")

	try:
		fs.serve_forever()
	except KeyboardInterrupt:
		fs.server_close()
		db.shutdown()
