INSERT INTO settings (key, value, description) VALUES ('probe_accounts', '[]',
 'Uji form: rekening uji milik unit per bank/e-wallet (JSON [{bank, number, name}]) — dipilih sesuai dropdown bank di form situs')
ON CONFLICT (key) DO NOTHING;
