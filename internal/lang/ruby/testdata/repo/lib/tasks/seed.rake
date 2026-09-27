require "csv"

namespace :db do
  task :seed_users do
    CSV.foreach("users.csv") { |row| User.create!(row.to_h) }
  end
end
