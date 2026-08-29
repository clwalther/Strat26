import json
import sqlite3
import datetime

with open("./players.json", "r", encoding="utf-8") as file:
	players = json.load(file)

class Database():
	def __init__(self, path):
		self.path = path

		try:
			connection = self.connection()
			cursor = connection.cursor()

			# Test connection and context to database
			cursor.execute("SELECT sqlite_version();")
			print(f"SQLlite version: {cursor.fetchall()[0][0]}")

			# Initiate database
			self._init_timetable(cursor)
			self._init_players(cursor)

			connection.commit()
			connection.close()

		except sqlite3.Error as error:
			print(f"DATABASE ERROR: FATAL: {error}")

	def _init_timetable(self, cursor) -> None:
		# cursor.execute("DROP TABLE IF EXISTS timetable") # <---- ONLY DEV
		cursor.execute("""CREATE TABLE IF NOT EXISTS timetable (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			status VARCHAR(100) NOT NULL,
			time TIMESTAMP DEFAULT CURRENT_TIMESTAMP NOT NULL
		);""")

		# Gurantee that the status is 'PAUSED' on startup
		cursor.execute("""INSERT INTO timetable (status) VALUES ('PAUSED')""")

	def _init_players(self, cursor) -> None:
		cursor.execute("DROP TABLE IF EXISTS players") # <---- ONLY DEV
		cursor.execute("""CREATE TABLE IF NOT EXISTS players (
			id INTEGER PRIMARY KEY AUTOINCREMENT,

			name VARCHAR(100) NOT NULL,
			team VARCHAR(100) NOT NULL,

			pac INT NOT NULL,
			sho INT NOT NULL,
			pas INT NOT NULL,
			dri INT NOT NULL,
			def INT NOT NULL,
			phy INT NOT NULL,
			dop INT NOT NULL DEFAULT 0,

			fatigue INT NOT NULL DEFAULT 0,

			punishment VARCHAR(100) DEFAULT NULL,
			punishment_time TIMESTAMP DEFAULT NULL,

			role VARCHAR(100) NOT NULL,
			position INT NOT NULL
		);""")

		# Check wheter table players is correctly populated
		cursor.execute('SELECT * FROM players;')

		if len(cursor.fetchall()) != 52:
			self._populate_players(cursor)

	def _populate_players(self, cursor) -> None:
		# Purge all players
		cursor.execute("DELETE FROM players;")

		# Repopulate players
		for player in players:
			cursor.execute(f"""INSERT INTO players
			(name, team, pac, sho, pas, dri, def, phy, role, position)
			VALUES (
				'{player['name']}',
				'{player['team']}',
				{player['pac']},
				{player['sho']},
				{player['pas']},
				{player['dri']},
				{player['def']},
				{player['phy']},
				'{player['role']}',
				{player['position']}
			);""")

	def connection(self) -> sqlite3.Connection:
		return sqlite3.connect(self.path)

	def shutdown(self) -> None:
		try:
			connection = self.connection()
			cursor = connection.cursor()

			# Gurantee that the status is 'PAUSED' when shutdown
			cursor.execute("""INSERT INTO timetable (status) VALUES ('PAUSED')""")

			connection.commit()
			connection.close()
		except sqlite3.Error as error:
			print(f"DATABASE ERROR: FATAL: {error}")

	# === Shortcuts ===
	# status
	def get_status(self) -> str:
		try:
			connection = self.connection()
			cursor = connection.cursor()

			cursor.execute("SELECT status FROM timetable ORDER BY id DESC LIMIT 1;")
			status = cursor.fetchall()[0][0]

			connection.commit()
			connection.close()

			return status

		except sqlite3.Error as error:
			print(f"DATABASE ERROR: FATAL: {error}")
			return None

	def set_status(self, status: str) -> int:
		try:
			connection = self.connection()
			cursor = connection.cursor()

			cursor.execute(f"INSERT INTO timetable (status) VALUES ('{status}');")

			connection.commit()
			connection.close()

		except sqlite3.Error as error:
			print(f"DATABASE ERROR: FATAL: {error}")
			return -1
		return 0

	# duration
	def get_duration(self) -> int:
		try:
			con = self.connection()
			cur = con.cursor()

			cur.execute("SELECT status, time FROM timetable ORDER BY id DESC;")

			duration = 0
			time_end = datetime.datetime.now(datetime.timezone.utc)

			for row in cur.fetchall():
				time_start = datetime.datetime.strptime(row[1], "%Y-%m-%d %H:%M:%S").replace(tzinfo=datetime.timezone.utc)

				if row[0] == "RUNNING":
					duration += (time_end - time_start).total_seconds()

				time_end = time_start

			con.commit()
			con.close()

			return int(duration)

		except sqlite3.Error as error:
			print(f"DATABASE ERROR: {error}")
			return None

	# database
	def get_version(self) -> str:
		try:
			connection = self.connection()
			cursor = connection.cursor()

			cursor.execute("SELECT sqlite_version();")
			version = cursor.fetchall()[0][0]

			connection.commit()
			connection.close()

			return f"SQLite {version}"

		except sqlite3.Error as error:
			print(f"DATABASE ERROR: FATAL: {error}")
			return None

	# players
	def get_players(self) -> list[dict]:
		try:
			connection = self.connection()
			cursor = connection.cursor()

			cursor.execute("SELECT * FROM players ORDER BY position ASC;")

			players = []
			for row in cursor.fetchall():
				players.append({
					"id": row[0],
					"name": row[1],
					"team": row[2],

					"pac": row[3],
					"sho": row[4],
					"pas": row[5],
					"dri": row[6],
					"def": row[7],
					"phy": row[8],
					"dop": row[9],

					"fatigue": row[10],

					"punish": row[11],
					"punish_time": row[12],

					"role": row[13],
					"position": row[14]
				})

			connection.commit()
			connection.close()

			return players

		except sqlite3.Error as error:
			print(f"DATABASE ERROR: FATAL: {error}")
			return None

	def set_positions(self, players) -> int:
		try:
			connection = self.connection()
			cursor = connection.cursor()

			for player in players:
				cursor.execute(f"""UPDATE players SET
					role = '{player['role']}',
					position = {player['position']}
				WHERE
					id = {player['id']};
				""")

			connection.commit()
			connection.close()

		except sqlite3.Error as error:
			print(f"DATABASE ERROR: FATAL: {error}")
			return -1
		return 0

	def set_player(self, player) -> int:
		try:
			con = self.connection()
			cur = con.cursor()

			cur.execute(f"""UPDATE players SET
					pac = {player['pac']},
					sho = {player['sho']},
					pas = {player['pas']},
					dri = {player['dri']},
					def = {player['def']},
					phy = {player['phy']},
					dop = {player['dop']}
				WHERE
					id = {player['id']};""")

			con.commit()
			con.close()

		except sqlite3.Error as error:
			print(f"DATABASE ERROR: FATAL: {error}")
			return -1
		return 0