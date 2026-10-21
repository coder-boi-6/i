-- name: CreateUser :one
INSERT INTO users (id, created_at, updated_at, email,hashed_password)
VALUES (
	gen_random_uuid(),NOW(),NOW(),$1 ,$2
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
-- name: UserbyEmail :one
select *
from users
where email = $1;
-- name: CreateRefreshRow :one
insert into refresh_tokens (token,created_at,updated_at,user_id,expires_at)
values($1,NOW(),NOW(),$2,$3)
returning *;
-- name: UserRefreshTokenThing :one
select users.*
from users inner join refresh_tokens on users.id = refresh_tokens.user_id
where refresh_tokens.token=$1
and revoked_at is null
and expires_at>NOW();
-- name: RevokeToken :exec
update refresh_tokens set revoked_at=NOW(),updated_at=NOW()
where refresh_tokens.token = $1;