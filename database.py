import json
import sqlite3
import datetime

with open("./players.json", "r", encoding="utf-8") as file:
	init_players = json.load(file)

def safe_access(func) -> callable:
	def wrapper(*args, **kwargs) -> any:
		try:
			db = args[0]
			connection = db.connection()
			cursor = connection.cursor()

			new_args = list(args)
			new_args.insert(1, cursor)

			result = func(*new_args, **kwargs)

			connection.commit()
			connection.close()

			return result
		except sqlite3.Error as error:
			print(f"DATABASE ERROR: FATAL: {error}")
			return -1
	return wrapper

class Database():
	def __init__(self, path):
		self.path = path

		version = self.get_version()

		if version != -1:
			print(f"SQLite Version: {version}")

		self._init_timetable()
		self._init_players()

	def connection(self) -> sqlite3.Connection:
		return sqlite3.connect(self.path)

	@safe_access
	def shutdown(self, cursor) -> None:
		# Gurantee that the status is 'PAUSED' when shutdown
		cursor.execute("""INSERT INTO timetable (status) VALUES ('PAUSED')""")

	@safe_access
	def get_version(self, cursor) -> str:
		cursor.execute("SELECT sqlite_version();")
		version = cursor.fetchall()[0][0]

		return version

	# Init
	@safe_access
	def _init_timetable(self, cursor) -> None:
		# cursor.execute("DROP TABLE IF EXISTS timetable") # <---- ONLY DEV
		cursor.execute("""CREATE TABLE IF NOT EXISTS timetable (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			status VARCHAR(100) NOT NULL,
			time TIMESTAMP DEFAULT CURRENT_TIMESTAMP NOT NULL
		);""")

		# Gurantee that the status is 'PAUSED' on startup
		cursor.execute("""INSERT INTO timetable (status) VALUES ('PAUSED')""")

	@safe_access
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

			punishment VARCHAR(100) DEFAULT 'NONE',
			punishment_time TIMESTAMP DEFAULT NULL,

			role VARCHAR(100) NOT NULL,
			position INT NOT NULL
		);""")

		# Check wheter table players is correctly populated
		cursor.execute('SELECT * FROM players;')

		players = cursor.fetchall()

		if len(players) != 52:
			cursor.execute("DELETE FROM players;")

		# Repopulate players
		for player in init_players:
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

	# Methods
	@safe_access
	def get_status(self, cursor) -> str:
		cursor.execute("SELECT status FROM timetable ORDER BY id DESC LIMIT 1;")
		status = cursor.fetchall()[0][0]

		return status

	@safe_access
	def set_status(self, cursor, status: str) -> int:
		cursor.execute(f"INSERT INTO timetable (status) VALUES ('{status}');")

		return 0

	@safe_access
	def get_duration(self, cursor) -> int:
		cursor.execute("SELECT status, time FROM timetable ORDER BY id DESC;")

		duration = 0
		time_end = datetime.datetime.now(datetime.timezone.utc)

		for row in cursor.fetchall():
			time_start = datetime.datetime.strptime(row[1], "%Y-%m-%d %H:%M:%S").replace(tzinfo=datetime.timezone.utc)

			if row[0] == "RUNNING":
				duration += (time_end - time_start).total_seconds()

			time_end = time_start

		return int(duration)

	# players
	@safe_access
	def get_players(self, cursor) -> list[dict]:
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

				"punishment": row[11],
				"punishment_time": (None if row[12] is None else
					(datetime.datetime.strptime(row[12], "%Y-%m-%d %H:%M:%S").replace(tzinfo=datetime.timezone.utc) -
					datetime.datetime.now(datetime.timezone.utc)).total_seconds()),

				"role": row[13],
				"position": row[14]
			})

		return players

	@safe_access
	def set_positions(self, cursor, players) -> int:
		for player in players:
			cursor.execute(f"""UPDATE players SET
				role = '{player['role']}',
				position = {player['position']}
			WHERE
				id = {player['id']};
			""")
		return 0

	@safe_access
	def set_player(self, cursor, player) -> int:
		cursor.execute(f"""UPDATE players SET
				pac = {player['pac']},
				sho = {player['sho']},
				pas = {player['pas']},
				dri = {player['dri']},
				def = {player['def']},
				phy = {player['phy']},
				dop = {player['dop']}
			WHERE
				id = {player['id']};""")

		return 0

	@safe_access
	def set_punishment(self, cursor, player) -> int:
		if player['punishment'] == "RED":
			minutes = 15
			cursor.execute(f"""UPDATE players SET
					punishment = '{player['punishment']}',
					punishment_time = '{(datetime.datetime.now(datetime.timezone.utc) + datetime.timedelta(minutes=minutes)).strftime("%Y-%m-%d %H:%M:%S")}',
					role = 'BENCH',
					position = -1
				WHERE
					id = {player['id']};""")
		elif player['punishment'] == "DOUBLE":
			minutes = 10
			cursor.execute(f"""UPDATE players SET
					punishment = '{player['punishment']}',
					punishment_time = '{(datetime.datetime.now(datetime.timezone.utc) + datetime.timedelta(minutes=minutes)).strftime("%Y-%m-%d %H:%M:%S")}',
					role = 'BENCH',
					position = -1
				WHERE
					id = {player['id']};""")
		elif player['punishment'] == "YELLOW":
			minutes = 5
			cursor.execute(f"""UPDATE players SET
					punishment = '{player['punishment']}',
					punishment_time = '{(datetime.datetime.now(datetime.timezone.utc) + datetime.timedelta(minutes=minutes)).strftime("%Y-%m-%d %H:%M:%S")}'
				WHERE
					id = {player['id']};""")
		else:
			cursor.execute(f"""UPDATE players SET
					punishment = '{player['punishment']}',
					punishment_time = NULL
				WHERE
					id = {player['id']};""")

		return 0
