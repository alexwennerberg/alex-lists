"""Add GraphQL mailing list webhook tables

Revision ID: 6deff4db061c
Revises: b3f299fe2115
Create Date: 2022-05-26 13:49:14.381048

"""

# revision identifiers, used by Alembic.
revision = '6deff4db061c'
down_revision = 'b3f299fe2115'

from alembic import op
import sqlalchemy as sa


def upgrade():
    op.execute("""
    CREATE TYPE list_webhook_event AS ENUM (
        'LIST_UPDATED',
        'LIST_DELETED',
        'EMAIL_RECEIVED',
        'PATCHSET_RECEIVED'
    );

    CREATE TABLE gql_list_wh_sub (
        id serial PRIMARY KEY,
        created timestamp NOT NULL,
        events list_webhook_event[] NOT NULL check (array_length(events, 1) > 0),
        url varchar NOT NULL,
        query varchar NOT NULL,

        auth_method auth_method NOT NULL check (auth_method in ('OAUTH2', 'INTERNAL')),
        token_hash varchar(128) check ((auth_method = 'OAUTH2') = (token_hash IS NOT NULL)),
        grants varchar,
        client_id uuid,
        expires timestamp check ((auth_method = 'OAUTH2') = (expires IS NOT NULL)),
        node_id varchar check ((auth_method = 'INTERNAL') = (node_id IS NOT NULL)),

        user_id integer NOT NULL references "user"(id),
        list_id integer references "list"(id) ON DELETE SET NULL
    );

    CREATE INDEX gql_list_wh_sub_token_hash_idx ON gql_list_wh_sub (token_hash);

    CREATE TABLE gql_list_wh_delivery (
        id serial PRIMARY KEY,
        uuid uuid NOT NULL,
        date timestamp NOT NULL,
        event list_webhook_event NOT NULL,
        subscription_id integer NOT NULL references gql_list_wh_sub(id) ON DELETE CASCADE,
        request_body varchar NOT NULL,
        response_body varchar,
        response_headers varchar,
        response_status integer
    );
    """)


def downgrade():
    op.execute("""
    DROP TABLE gql_list_wh_delivery;
    DROP INDEX gql_list_wh_sub_token_hash_idx;
    DROP TABLE gql_list_wh_sub;
    DROP TYPE list_webhook_event;
    """)
