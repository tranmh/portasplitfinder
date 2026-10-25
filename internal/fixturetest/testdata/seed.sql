-- Shared integration seed; IDs match internal/fixturetest/fixtures.go.
INSERT INTO products (id, name, model) VALUES
  ('11111111-1111-1111-1111-111111111111', 'Midea PortaSplit', 'PortaSplit 12000'),
  ('22222222-2222-2222-2222-222222222222', 'Midea PortaSplit XL', 'PortaSplit 14000')
ON CONFLICT DO NOTHING;
INSERT INTO sources (key, name, type, base_url) VALUES
  ('mock', 'Mock Store', 'store', 'https://mock.example'),
  ('mockonline', 'Mock Online', 'online', 'https://online.example')
ON CONFLICT DO NOTHING;
INSERT INTO stores (id, source_key, name, street, plz, city, lat, lon) VALUES
  ('33333333-3333-3333-3333-333333333333', 'mock', 'Mock Göppingen', 'Hauptstr. 1', '73035', 'Göppingen', 48.7036, 9.6527)
ON CONFLICT DO NOTHING;
INSERT INTO availability_snapshots (source_key, store_id, product_id, in_stock, quantity, price_cents, url, checked_at) VALUES
  ('mock', '33333333-3333-3333-3333-333333333333', '11111111-1111-1111-1111-111111111111', false, 0, 59900, 'https://mock.example/p', '2026-01-01T00:00:00Z'),
  ('mock', '33333333-3333-3333-3333-333333333333', '11111111-1111-1111-1111-111111111111', true, 3, 59900, 'https://mock.example/p', '2026-01-02T03:04:05Z'),
  ('mockonline', NULL, '11111111-1111-1111-1111-111111111111', true, NULL, 61900, 'https://online.example/p', '2026-01-02T03:04:05Z');
INSERT INTO subscriptions (id, email, plz, radius_km, confirm_token, unsub_token, confirmed_at) VALUES
  ('44444444-4444-4444-4444-444444444444', 'user@example.com', '73035', 50, 'confirm-token', 'unsub-token', '2026-01-02T03:04:05Z')
ON CONFLICT DO NOTHING;
