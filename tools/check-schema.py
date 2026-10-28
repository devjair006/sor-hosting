#!/usr/bin/env python3
"""Exercise SQLite constraints without substituting for the Go integration suite."""
from pathlib import Path
import sqlite3
import unittest
ROOT=Path(__file__).resolve().parents[1]
class SchemaTests(unittest.TestCase):
 def setUp(self):
  self.db=sqlite3.connect(':memory:');self.db.execute('PRAGMA foreign_keys=ON');[self.db.executescript(p.read_text()) for p in sorted((ROOT/'backend/migrations').glob('*.sql'))]
 def tearDown(self): self.db.close()
 def site(self,id='site1',domain='a.example',state='active'):
  self.db.execute('INSERT INTO sites(id,server_id,name,domain,state,metadata,created_at,updated_at) VALUES(?,?,?,?,?,?,?,?)',(id,'local','Example',domain,state,'{}','now','now'))
 def test_domain_is_unique_per_server(self):
  self.site()
  with self.assertRaises(sqlite3.IntegrityError):self.site('site2')
 def test_foreign_server_cannot_own_site(self):
  with self.assertRaises(sqlite3.IntegrityError):self.db.execute("INSERT INTO sites(id,server_id,name,domain,state,metadata,created_at,updated_at) VALUES('s','foreign','X','a.example','active','{}','now','now')")
 def test_unknown_site_state_rejected(self):
  with self.assertRaises(sqlite3.IntegrityError):self.site(state='invented')
 def test_pending_ssl_is_distinct_from_pending_dns(self):
  self.site(state='pending_ssl');self.assertEqual(self.db.execute('SELECT state FROM sites').fetchone()[0],'pending_ssl')
 def test_session_requires_user_and_cascades(self):
  self.db.execute("INSERT INTO users VALUES('u','a@example.com','hash','admin','now')");self.db.execute("INSERT INTO sessions VALUES('token-hash','u',1,1,2)");self.db.execute("DELETE FROM users WHERE id='u'");self.assertEqual(self.db.execute('SELECT count(*) FROM sessions').fetchone()[0],0)
 def test_job_key_cannot_create_duplicate_operation(self):
  self.db.execute("INSERT INTO users VALUES('u','a@example.com','hash','admin','now')")
  values=('job1','u','site.install','site1','queued','pending','encrypted','same-key','digest','{}','','now','now')
  self.db.execute('INSERT INTO jobs VALUES('+','.join('?'*13)+')',values)
  with self.assertRaises(sqlite3.IntegrityError):self.db.execute('INSERT INTO jobs VALUES('+','.join('?'*13)+')',('job2',)+values[1:])
 def test_generated_query_sources_prepare(self):
  import re
  source=(ROOT/'backend/queries/panel.sql').read_text()
  for query in re.split(r'-- name: [^\n]+\n',source)[1:]:
   query=query.strip()
   self.db.execute('EXPLAIN '+query,[None]*query.count('?')).fetchall()
 def test_agent_journal_sources_prepare(self):
  import re
  with sqlite3.connect(':memory:') as db:
   [db.executescript(p.read_text()) for p in sorted((ROOT/'agent/migrations').glob('*.sql'))]
   for query in re.split(r'-- name: [^\n]+\n',(ROOT/'agent/queries/journal.sql').read_text())[1:]:
    query=query.strip();db.execute('EXPLAIN '+query,[None]*query.count('?')).fetchall()
 def test_container_has_one_association(self):
  self.site();self.site('site2','b.example');self.db.execute("INSERT INTO site_containers VALUES('site1','container1','web')")
  with self.assertRaises(sqlite3.IntegrityError):self.db.execute("INSERT INTO site_containers VALUES('site2','container1','web')")
if __name__=='__main__':unittest.main(verbosity=2)
