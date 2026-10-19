-- name: CreateUser :one
INSERT INTO users (id, created_at, updated_at, email)
VALUES (
	gen_random_uuid(),NOW(),NOW(),$1
)
RETURNING *;
-- name: DeleteAllUsers :exec
delete from users;
-- name: CreateChirp :one
insert into chirps (id,created_at,updated_at,body,user_id)
values(
    gen_random_uuid(),NOW(),NOW(),$1,$2
)
RETURNING *;
-- name: RetrieveAllChirps :many
select *
from chirps
order by created_at asc;
-- name: GetChirp :one
SELECT * FROM chirps
WHERE id = $1;
