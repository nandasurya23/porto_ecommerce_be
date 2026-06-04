INSERT INTO categories (name, slug, description, is_active)
VALUES
  ('Running Shoes', 'running-shoes', 'Shoes for running', true),
  ('Lifestyle Shoes', 'lifestyle-shoes', 'Daily casual shoes', true),
  ('Training Shoes', 'training-shoes', 'Shoes for training', true),
  ('Football Shoes', 'football-shoes', 'Shoes for football', true),
  ('Sandals', 'sandals', 'Casual sandals', true),
  ('Limited Edition', 'limited-edition', 'Limited footwear releases', true)
ON CONFLICT (slug) DO NOTHING;
