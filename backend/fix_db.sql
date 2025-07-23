-- Fix database foreign key constraint issues

-- First check if there are any problematic records
SELECT ts.id, ts.name, ts.environment_id, e.id as env_exists 
FROM test_suites ts 
LEFT JOIN environments e ON ts.environment_id = e.id 
WHERE e.id IS NULL AND ts.environment_id IS NOT NULL AND ts.environment_id != 0;

-- Find the first available environment ID
SELECT MIN(id) as min_env_id FROM environments WHERE status = 1;

-- Update problematic test_suites records to use the first available environment
-- This assumes there's at least one environment with status = 1
UPDATE test_suites 
SET environment_id = (SELECT MIN(id) FROM environments WHERE status = 1)
WHERE environment_id NOT IN (SELECT id FROM environments) 
   OR environment_id IS NULL 
   OR environment_id = 0;

-- Check if the fix worked
SELECT COUNT(*) as problematic_records 
FROM test_suites ts 
LEFT JOIN environments e ON ts.environment_id = e.id 
WHERE e.id IS NULL;