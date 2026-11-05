DROP TABLE IF EXISTS form_probes;
DELETE FROM settings WHERE key LIKE 'probe\_%';
