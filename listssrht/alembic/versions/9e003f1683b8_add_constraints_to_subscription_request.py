"""add constraints to subscription request

Revision ID: 9e003f1683b8
Revises: 0147eed31dae
Create Date: 2024-12-06 09:52:58.325282

"""

# revision identifiers, used by Alembic.
revision = '9e003f1683b8'
down_revision = '0147eed31dae'

from alembic import op
import sqlalchemy as sa


def upgrade():
    op.execute("""
    DELETE FROM subscription_request
    WHERE email IS NULL OR confirmation_hash IS NULL;

    DELETE FROM subscription_request WHERE id IN (
        SELECT MIN(id) FROM subscription_request
        GROUP BY (list_id, email) HAVING count(*) > 1
    );

    ALTER TABLE subscription_request
    ALTER COLUMN email SET NOT NULL;

    ALTER TABLE subscription_request
    ALTER COLUMN confirmation_hash SET NOT NULL;

    ALTER TABLE subscription_request
    ADD CONSTRAINT sr_list_id_email_unique
    UNIQUE (list_id, email);
    """)


def downgrade():
    op.execute("""
    ALTER TABLE subscription_request
    DROP CONSTRAINT sr_list_id_email_unique;

    ALTER TABLE subscription_request
    ALTER COLUMN confirmation_hash DROP NOT NULL;

    ALTER TABLE subscription_request
    ALTER COLUMN email DROP NOT NULL;
    """)
