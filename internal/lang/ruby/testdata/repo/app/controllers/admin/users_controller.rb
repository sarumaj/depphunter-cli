module Admin
  class UsersController < ApplicationController
    def index
      @users = User.all
      @roles = Role.all
    rescue ActiveRecord::RecordNotFound, ::Admin::Missing
      head :not_found
    end
  end
end
