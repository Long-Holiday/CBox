"""Database connection and session factory for SQLite."""

from pathlib import Path
from typing import Generator
from sqlalchemy import create_engine
from sqlalchemy.orm import DeclarativeBase, Session, sessionmaker

from cbox.utils.paths import default_paths


class Base(DeclarativeBase):
    pass


class Database:
    """Manages SQLite database connections and schema initialization."""

    def __init__(self, db_path: Path | str | None = None):
        if db_path:
            self.db_path = Path(db_path)
        else:
            self.db_path = default_paths.db_path

        self.db_path.parent.mkdir(parents=True, exist_ok=True, mode=0o700)
        self.url = f"sqlite:///{self.db_path}"
        self.engine = create_engine(
            self.url,
            connect_args={"check_same_thread": False},
            echo=False,
        )
        self.SessionLocal = sessionmaker(autocommit=False, autoflush=False, bind=self.engine)

    def init_db(self) -> None:
        """Create all tables if they do not exist."""
        Base.metadata.create_all(bind=self.engine)

    def get_session(self) -> Generator[Session, None, None]:
        """Provide a transactional database session."""
        session = self.SessionLocal()
        try:
            yield session
            session.commit()
        except Exception:
            session.rollback()
            raise
        finally:
            session.close()


default_db = Database()
