db = db.getSiblingDB("shortener");
db.createUser({
  user: "admin",
  pwd: "password",
  roles: [
    {
      role: "readWrite",
      db: "shortener"
    }
  ]
});