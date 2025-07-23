-- Disable foreign key checks temporarily
SET FOREIGN_KEY_CHECKS = 0;

-- Check the current data issues
SELECT COUNT(*) as total_test_suites FROM test_suites;
SELECT COUNT(*) as total_environments FROM environments;

-- Check problematic records
SELECT ts.id, ts.name, ts.environment_id 
FROM test_suites ts 
WHERE ts.environment_id NOT IN (SELECT id FROM environments)
   OR ts.environment_id IS NULL 
   OR ts.environment_id = 0
LIMIT 10;

-- Get the first available environment
SELECT MIN(id) as first_env_id FROM environments WHERE status = 1;

-- Update all problematic test suites to use the first environment
UPDATE test_suites 
SET environment_id = (SELECT MIN(id) FROM environments WHERE status = 1) 
WHERE environment_id NOT IN (SELECT id FROM environments) 
   OR environment_id IS NULL 
   OR environment_id = 0;

-- Verify the fix
SELECT COUNT(*) as remaining_problematic_records 
FROM test_suites ts 
WHERE ts.environment_id NOT IN (SELECT id FROM environments);

-- Re-enable foreign key checks
SET FOREIGN_KEY_CHECKS = 1;