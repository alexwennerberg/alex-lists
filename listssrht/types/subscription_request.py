import sqlalchemy as sa
from srht.database import Base
import base64
import os

class SubscriptionRequest(Base):
    __tablename__ = 'subscription_request'

    id = sa.Column(sa.Integer, primary_key=True)
    email = sa.Column(sa.Unicode(512))
    confirmation_hash = sa.Column(sa.String(128))

    list_id = sa.Column(sa.Integer,
            sa.ForeignKey('list.id', ondelete="CASCADE"),
            nullable=False)
    list = sa.orm.relationship('List')

    def __init__(self, email, list_id):
        self.email = email
        self.list_id = list_id
        self.gen_confirmation_hash()

    def gen_confirmation_hash(self):
        self.confirmation_hash = (
            base64.urlsafe_b64encode(os.urandom(18))
        ).decode('utf-8')
        return self.confirmation_hash

    def __repr__(self):
        return '<SubscriptionRequest {} {} -> list {}>'.format(
                self.id, self.email, self.list_id)

    def to_dict(self):
        return {
            "id": self.id,
            "email": self.email,
            "list": self.list.to_dict(short=True),
        }
