-- [mok] Fork-only migration (see FORK.md): system-generated invite codes are 4 characters
-- long instead of 12 (affiliateCodeLength in internal/repository/affiliate_repo.go). This
-- gives every existing user a new random 4-character code from the same alphabet.
--
-- Codes an admin set by hand (aff_code_custom) are kept, and so are codes that are already
-- 4 characters long. The pool holds every 4-character code that is not taken, in random
-- order, so the new codes are unique among themselves and against the kept ones. With more
-- users than free codes (about a million), the users left over keep their old code. Invite
-- relations (inviter_id) do not depend on the code and stay as they are.
WITH chars AS (
    SELECT substr('ABCDEFGHJKLMNPQRSTUVWXYZ23456789', i, 1) AS c
    FROM generate_series(1, 32) AS i
),
pool AS (
    SELECT codes.code, row_number() OVER (ORDER BY random()) AS rn
    FROM (
        SELECT c1.c || c2.c || c3.c || c4.c AS code
        FROM chars c1, chars c2, chars c3, chars c4
    ) codes
    WHERE NOT EXISTS (SELECT 1 FROM user_affiliates taken WHERE taken.aff_code = codes.code)
),
targets AS (
    SELECT user_id, row_number() OVER (ORDER BY user_id) AS rn
    FROM user_affiliates
    WHERE aff_code_custom = false
      AND char_length(aff_code) <> 4
)
UPDATE user_affiliates ua
SET aff_code = pool.code,
    updated_at = NOW()
FROM targets
JOIN pool ON pool.rn = targets.rn
WHERE ua.user_id = targets.user_id;
