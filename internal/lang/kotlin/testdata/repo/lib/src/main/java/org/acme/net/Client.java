package org.acme.net;

import com.example.app.model.User;
import com.example.app.util.StringsKt;

public class Client {
    public String greet(User u) { return StringsKt.shout(u.getName()); }
}
