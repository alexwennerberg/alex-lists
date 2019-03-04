import sqlalchemy as sa
from srht.config import cfg
from srht.database import DbSession, db
if not hasattr(db, "session"):
    # Initialize the database if not already configured (for running daemon)
    db = DbSession(cfg("lists.sr.ht", "connection-string"))
    import listssrht.types
    db.init()
from srht.webhook import Event
from srht.webhook.celery import CeleryWebhook, worker

class ListWebhook(CeleryWebhook):
    events = [
        Event("post:received", "lists:read"),
        Event("list:update", "lists:read"),
    ]

    list_id = sa.Column(sa.Integer, sa.ForeignKey("list.id"), nullable=False)
    list = sa.orm.relationship("List")

class UserWebhook(CeleryWebhook):
    events = [
        Event("email:received", "emails:read"),
        Event("list:create", "lists:read"),
        Event("subscription:create", "subs:read"),
        Event("subscription:remove", "subs:read"),
    ]
