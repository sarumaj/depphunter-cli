<?php

namespace App\Models;

class User
{
    use HasName;

    public static function find(int $id): ?self
    {
        return null;
    }
}
