import sqlalchemy as sa
from srht.config import cfg
from srht.database import DbSession, db
if not hasattr(db, "session"):
    # Initialize the database if not already configured (for running daemon)
    db = DbSession(cfg("lists.sr.ht", "connection-string"))
    import listssrht.types
    db.init()
from srht.webhook import Event
from srht.webhook.celery import CeleryWebhook, make_worker

worker = make_worker(broker=cfg("lists.sr.ht", "webhooks"))

class ListWebhook(CeleryWebhook):
    events = [
        Event("post:received", "lists:read"),
        Event("list:update", "lists:read"),
        Event("list:delete", "lists:read"),
        Event("patchset:received", "patches:read"),
        Event("patchset:update", "patches:read"), # TODO: Deliver
    ]

    list_id = sa.Column(sa.Integer, sa.ForeignKey("list.id"))
    list = sa.orm.relationship("List")

class UserWebhook(CeleryWebhook):
    events = [
        Event("email:received", "emails:read"),
        Event("list:create", "lists:read"),
        Event("subscription:create", "subs:read"),
        Event("subscription:remove", "subs:read"),
    ]
